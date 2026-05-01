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
	"unicode/utf8"

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
	stdin bool   // true: pipe text via stdin; false: pass text as final argv
	path  string // resolved by tryNativeClipboard before nativeRun is called
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
// Skips (without writing) when stderr is not a TTY. Payloads over the safe
// limit are truncated to the *last* osc52SafeLimit bytes — for build/task
// logs the tail is where the error usually lives, so it is the most useful
// part to recover. The view-layer toast tells the user that truncation
// occurred and the 'w' keybinding is still available for the full log.
func tryOSC52(text string) (copyMethod, bool) {
	if !osc52IsTTY() {
		return copyMethodUnknown, false
	}
	if len(text) > osc52SafeLimit {
		start := len(text) - osc52SafeLimit
		// Walk forward to the next valid UTF-8 rune start so we never split
		// a multi-byte rune and produce U+FFFD on paste.
		for start < len(text) && !utf8.RuneStart(text[start]) {
			start++
		}
		text = text[start:]
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
		// Can't verify the socket. Modern user sessions always set
		// XDG_RUNTIME_DIR, so its absence reliably indicates wl-copy will
		// fail. Skip rather than waste a 2 s timeout on a doomed attempt.
		return false
	}
	return statFile(rd+"/"+wd) == nil
}

// tryNativeClipboard runs the candidate ladder, returning true on the first
// success. exec.ErrWaitDelay is treated as success: xclip/xsel daemonize and
// hold the inherited stdin pipe open, so the foreground process exits cleanly
// while WaitDelay force-closes pipes — the clipboard already received the data.
func tryNativeClipboard(parent context.Context, cands []nativeCmd, text string) bool {
	for _, c := range cands {
		p, err := lookPath(c.name)
		if err != nil {
			continue
		}
		c.path = p
		if err := nativeRun(parent, c, text); err == nil || errors.Is(err, exec.ErrWaitDelay) {
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
	// c.path was resolved by tryNativeClipboard via lookPath so
	// exec.CommandContext does not repeat the PATH search and there is no
	// TOCTOU between lookup and invocation.
	cmd := exec.CommandContext(ctx, c.path, argv...) //nolint:gosec
	cmd.WaitDelay = time.Second
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if c.stdin {
		cmd.Stdin = strings.NewReader(text)
	}
	return cmd.Run()
}
