package actions

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"golang.org/x/term"
)

// osc52SafeLimit is the largest plaintext size where OSC 52 is reliably
// accepted across terminals. xterm caps the OSC 52 sequence around 100 KB
// (~74 KB of plaintext); st and older kitty are smaller. 64 KB is a
// safe-everywhere ceiling — above it, fall back to a native tool or to the
// "save to file" keybinding.
const osc52SafeLimit = 64 * 1024

// copyMethod identifies which backend handled a successful copy.
type copyMethod int

const (
	copyMethodUnknown     copyMethod = iota
	copyMethodNative                 // local clipboard tool (pbcopy/xclip/...)
	copyMethodOSC52                  // raw OSC 52 escape sequence
	copyMethodOSC52Tmux              // OSC 52 wrapped for tmux pass-through
	copyMethodOSC52Screen            // OSC 52 chunked through GNU screen DCS
)

// Test seams. Production code uses the real OS implementations.
var (
	osc52Out   io.Writer = os.Stderr
	osc52IsTTY           = func() bool { return term.IsTerminal(int(os.Stderr.Fd())) }
	getEnv               = os.Getenv
	statFile             = func(name string) error { _, err := os.Stat(name); return err }
	runtimeOS            = runtime.GOOS
	lookPath             = exec.LookPath
	nativeRun            = realNativeRun
)

// nativeCmd is a single clipboard backend invocation recipe.
type nativeCmd struct {
	name  string
	args  []string
	stdin bool // true: pipe text via stdin; false: pass text as final argv
}

// copyToClipboard copies text using the most appropriate backend.
//
// Order is environment-aware:
//   - In an SSH session (SSH_TTY/SSH_CONNECTION set), OSC 52 first — local
//     tools would target the *server's* clipboard, not the user's.
//   - Otherwise, native first, then OSC 52 fallback.
//
// Each native attempt is bounded by a per-attempt timeout. OSC 52 sequences
// are emitted to stderr (not stdout) so they cannot race with a BubbleTea
// inline renderer that owns stdout. Both streams reach the same TTY.
func copyToClipboard(ctx context.Context, text string) (copyMethod, error) {
	cands := nativeCandidates()
	if inSSH() {
		if m, ok := tryOSC52(text); ok {
			return m, nil
		}
		if tryNativeClipboard(ctx, cands, text) {
			return copyMethodNative, nil
		}
	} else {
		if tryNativeClipboard(ctx, cands, text) {
			return copyMethodNative, nil
		}
		if m, ok := tryOSC52(text); ok {
			return m, nil
		}
	}
	return copyMethodUnknown, errors.New("no clipboard tool available")
}

func inSSH() bool {
	return getEnv("SSH_TTY") != "" || getEnv("SSH_CONNECTION") != ""
}

// tryOSC52 emits an OSC 52 escape sequence using the go-osc52 library,
// auto-wrapping for tmux ($TMUX) or GNU screen ($STY + $TERM=screen*).
//
// Skips (without writing) when stderr is not a TTY or when the payload exceeds
// osc52SafeLimit — most terminals silently drop oversized sequences, so it is
// better to surface failure than to claim success.
func tryOSC52(text string) (copyMethod, bool) {
	if !osc52IsTTY() {
		return copyMethodUnknown, false
	}
	if len(text) > osc52SafeLimit {
		return copyMethodUnknown, false
	}

	seq := osc52.New(text)
	method := copyMethodOSC52
	switch {
	case getEnv("TMUX") != "":
		seq = seq.Tmux()
		method = copyMethodOSC52Tmux
	case getEnv("STY") != "" && strings.HasPrefix(getEnv("TERM"), "screen"):
		seq = seq.Screen()
		method = copyMethodOSC52Screen
	}
	if _, err := seq.WriteTo(osc52Out); err != nil {
		return copyMethodUnknown, false
	}
	return method, true
}

// nativeCandidates returns the native-tool ladder, ordered for the current
// environment. Tools whose runtime requirements are clearly absent (e.g.
// wl-copy without a live Wayland socket) are omitted to avoid wasted
// attempts and timeout-bounded hangs.
func nativeCandidates() []nativeCmd {
	var c []nativeCmd

	switch runtimeOS {
	case "darwin":
		c = append(c, nativeCmd{name: "pbcopy", stdin: true})
	case "linux":
		if isWaylandLive() {
			c = append(c, nativeCmd{name: "wl-copy", stdin: true})
		}
		if getEnv("DISPLAY") != "" {
			c = append(c,
				nativeCmd{name: "xclip", args: []string{"-selection", "clipboard"}, stdin: true},
				nativeCmd{name: "xsel", args: []string{"--clipboard", "--input"}, stdin: true},
			)
		}
		c = append(c, nativeCmd{
			name: "qdbus",
			args: []string{
				"org.kde.klipper", "/klipper",
				"org.kde.klipper.klipper.setClipboardContents",
			},
			stdin: false,
		})
		c = append(c, nativeCmd{name: "termux-clipboard-set", stdin: true})
	}
	return c
}

func isWaylandLive() bool {
	wd := getEnv("WAYLAND_DISPLAY")
	if wd == "" {
		return false
	}
	if strings.HasPrefix(wd, "/") {
		return statFile(wd) == nil
	}
	rd := getEnv("XDG_RUNTIME_DIR")
	if rd == "" {
		// No way to verify the socket; proceed optimistically. The per-attempt
		// timeout bounds any misfire.
		return true
	}
	return statFile(rd+"/"+wd) == nil
}

// tryNativeClipboard runs the candidate ladder, returning true on the first
// success. exec.ErrWaitDelay is treated as success: xclip/xsel daemonize and
// hold the inherited stdin pipe open, so the foreground process exits cleanly
// while WaitDelay force-closes pipes — the clipboard already received the data.
func tryNativeClipboard(parent context.Context, cands []nativeCmd, text string) bool {
	for _, c := range cands {
		if _, err := lookPath(c.name); err != nil {
			continue
		}
		err := nativeRun(parent, c, text)
		if err == nil || errors.Is(err, exec.ErrWaitDelay) {
			return true
		}
	}
	return false
}

// realNativeRun runs a single clipboard tool with a tight per-attempt timeout
// and WaitDelay so background-forking tools cannot wedge the caller.
func realNativeRun(parent context.Context, c nativeCmd, text string) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	argv := append([]string{}, c.args...)
	if !c.stdin {
		argv = append(argv, text)
	}
	cmd := exec.CommandContext(ctx, c.name, argv...) //nolint:gosec
	cmd.WaitDelay = time.Second
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if c.stdin {
		cmd.Stdin = strings.NewReader(text)
	}
	return cmd.Run()
}
