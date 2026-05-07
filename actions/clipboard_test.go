package actions

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"
)

type seamOverrides struct {
	env        map[string]string
	osc52Out   io.Writer
	osc52IsTTY *bool
	runtimeOS  string
	nativeRun  func(ctx context.Context, c nativeCmd, text string) error
	lookPath   func(string) (string, error)
	statFile   func(string) error
}

func withSeams(t *testing.T, o seamOverrides) {
	t.Helper()
	origEnv := getEnv
	origOut := osc52Out
	origIsTTY := osc52IsTTY
	origOS := runtimeOS
	origRun := nativeRun
	origLook := lookPath
	origStat := statFile

	t.Cleanup(func() {
		getEnv = origEnv
		osc52Out = origOut
		osc52IsTTY = origIsTTY
		runtimeOS = origOS
		nativeRun = origRun
		lookPath = origLook
		statFile = origStat
	})

	if o.env != nil {
		getEnv = func(k string) string { return o.env[k] }
	}
	if o.osc52Out != nil {
		osc52Out = o.osc52Out
	}
	if o.osc52IsTTY != nil {
		v := *o.osc52IsTTY
		osc52IsTTY = func() bool { return v }
	}
	if o.runtimeOS != "" {
		runtimeOS = o.runtimeOS
	}
	if o.nativeRun != nil {
		nativeRun = o.nativeRun
	}
	if o.lookPath != nil {
		lookPath = o.lookPath
	}
	if o.statFile != nil {
		statFile = o.statFile
	}
}

func boolPtr(b bool) *bool { return &b }

// --- OSC 52 emission ---

func TestTryOSC52_DefaultEmitsRawSequence(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env:        map[string]string{},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
	})

	method, ok := tryOSC52("hello")
	if !ok || method != copyMethodOSC52 {
		t.Fatalf("expected raw OSC 52, got method=%v ok=%v", method, ok)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "\x1b]52;c;") {
		t.Errorf("expected OSC 52 prefix, got %q", got)
	}
	if !strings.HasSuffix(got, "\x07") {
		t.Errorf("expected BEL terminator, got %q", got)
	}
}

func TestTryOSC52_TmuxWrapsWithDCS(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env:        map[string]string{"TMUX": "/tmp/tmux-1000/default,1234,0"},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
	})

	method, ok := tryOSC52("hello")
	if !ok || method != copyMethodOSC52Tmux {
		t.Fatalf("expected tmux-wrapped OSC 52, got method=%v ok=%v", method, ok)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "\x1bPtmux;\x1b") {
		t.Errorf("expected tmux DCS prefix, got %q", got)
	}
	if !strings.HasSuffix(got, "\x1b\\") {
		t.Errorf("expected ST terminator, got %q", got)
	}
}

func TestTryOSC52_ScreenWrapsWithDCS(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env: map[string]string{
			"STY":  "12345.pts-0.host",
			"TERM": "screen.xterm-256color",
		},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
	})

	method, ok := tryOSC52("hello")
	if !ok || method != copyMethodOSC52Screen {
		t.Fatalf("expected screen-wrapped OSC 52, got method=%v ok=%v", method, ok)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "\x1bP") {
		t.Errorf("expected screen DCS prefix, got %q", got)
	}
}

func TestTryOSC52_SkipsWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env:        map[string]string{},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(false),
	})

	method, ok := tryOSC52("hello")
	if ok || method != copyMethodUnknown {
		t.Fatalf("expected skip when not a TTY, got method=%v ok=%v", method, ok)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when not TTY, wrote %d bytes", buf.Len())
	}
}

func TestTryOSC52_TruncatesOversizedPayloadToTail(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env:        map[string]string{},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
	})

	// Build a payload whose tail is identifiable: padding + marker.
	tail := "TAIL_MARKER_" + strings.Repeat("z", 100)
	pad := strings.Repeat("a", osc52SafeLimit)
	big := pad + tail

	method, ok := tryOSC52(big)
	if !ok || method != copyMethodOSC52 {
		t.Fatalf("expected OSC 52 success with truncation, got method=%v ok=%v", method, ok)
	}
	if buf.Len() == 0 {
		t.Fatal("expected truncated emission, got nothing")
	}
	// Decode the emitted base64 payload and assert the kept portion ends
	// with the tail marker (the head is what got dropped). A substring
	// check on the raw base64 would be unreliable: the tail's encoding
	// shifts depending on whether it starts on a 3-byte block boundary.
	decoded := decodeOSC52(t, buf.String())
	if !strings.HasSuffix(decoded, tail) {
		t.Errorf("expected decoded payload to end with tail marker, got %d bytes (last %d: %q)", len(decoded), len(tail), lastN(decoded, len(tail)))
	}
}

func TestTryOSC52_TruncationRespectsRuneBoundaries(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env:        map[string]string{},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
	})

	// "é" is 2 bytes (0xC3 0xA9) in UTF-8. Construct a payload such that
	// the naive byte-cut at len-osc52SafeLimit lands on the *continuation*
	// byte of a single "é": ASCII padding of length osc52SafeLimit-1, then
	// "é", then ASCII padding of length osc52SafeLimit-1. Total length is
	// 2*osc52SafeLimit, so the cut at index osc52SafeLimit lands on byte
	// 0xA9 (mid-rune).
	body := strings.Repeat("a", osc52SafeLimit-1) + "é" + strings.Repeat("a", osc52SafeLimit-1)
	method, ok := tryOSC52(body)
	if !ok || method != copyMethodOSC52 {
		t.Fatalf("expected OSC 52 success, got method=%v ok=%v", method, ok)
	}
	decoded := decodeOSC52(t, buf.String())
	if !utf8Valid(decoded) {
		t.Errorf("expected truncation to respect rune boundaries, got invalid UTF-8")
	}
	// The cut walked forward past the continuation byte, so the kept
	// payload is one byte shorter than osc52SafeLimit.
	if len(decoded) != osc52SafeLimit-1 {
		t.Errorf("expected decoded length %d after skipping continuation byte, got %d", osc52SafeLimit-1, len(decoded))
	}
}

// --- native candidate selection ---

func TestNativeCandidates_LinuxWaylandLive(t *testing.T) {
	withSeams(t, seamOverrides{
		env: map[string]string{
			"WAYLAND_DISPLAY": "wayland-0",
			"XDG_RUNTIME_DIR": "/run/user/1000",
		},
		runtimeOS: "linux",
		statFile:  func(string) error { return nil }, // socket exists
	})

	names := candNames(nativeCandidates())
	if !contains(names, "wl-copy") {
		t.Errorf("expected wl-copy in candidates, got %v", names)
	}
	if contains(names, "pbcopy") {
		t.Errorf("did not expect pbcopy on linux, got %v", names)
	}
}

func TestNativeCandidates_LinuxWaylandSocketMissing(t *testing.T) {
	withSeams(t, seamOverrides{
		env: map[string]string{
			"WAYLAND_DISPLAY": "wayland-0",
			"XDG_RUNTIME_DIR": "/run/user/1000",
		},
		runtimeOS: "linux",
		statFile:  func(string) error { return errors.New("no such file") },
	})

	names := candNames(nativeCandidates())
	if contains(names, "wl-copy") {
		t.Errorf("expected wl-copy excluded when socket missing, got %v", names)
	}
}

func TestNativeCandidates_LinuxWaylandNoRuntimeDir(t *testing.T) {
	withSeams(t, seamOverrides{
		env: map[string]string{
			"WAYLAND_DISPLAY": "wayland-0",
			// XDG_RUNTIME_DIR intentionally unset.
		},
		runtimeOS: "linux",
	})

	names := candNames(nativeCandidates())
	if contains(names, "wl-copy") {
		t.Errorf("expected wl-copy excluded when XDG_RUNTIME_DIR unset, got %v", names)
	}
}

func TestNativeCandidates_LinuxX11Only(t *testing.T) {
	withSeams(t, seamOverrides{
		env:       map[string]string{"DISPLAY": ":0"},
		runtimeOS: "linux",
	})

	names := candNames(nativeCandidates())
	if !contains(names, "xclip") {
		t.Errorf("expected xclip when DISPLAY set, got %v", names)
	}
	if !contains(names, "xsel") {
		t.Errorf("expected xsel when DISPLAY set, got %v", names)
	}
	if contains(names, "wl-copy") {
		t.Errorf("did not expect wl-copy without WAYLAND_DISPLAY, got %v", names)
	}
}

func TestNativeCandidates_LinuxNoDisplay(t *testing.T) {
	withSeams(t, seamOverrides{
		env:       map[string]string{},
		runtimeOS: "linux",
	})

	names := candNames(nativeCandidates())
	if contains(names, "xclip") || contains(names, "wl-copy") {
		t.Errorf("expected no graphical clipboard tools without DISPLAY/WAYLAND_DISPLAY, got %v", names)
	}
}

func TestNativeCandidates_DarwinAlwaysHasPbcopy(t *testing.T) {
	withSeams(t, seamOverrides{
		env:       map[string]string{},
		runtimeOS: "darwin",
	})

	names := candNames(nativeCandidates())
	if !contains(names, "pbcopy") {
		t.Errorf("expected pbcopy on darwin, got %v", names)
	}
	if contains(names, "xclip") || contains(names, "wl-copy") {
		t.Errorf("did not expect linux tools on darwin, got %v", names)
	}
}

// --- ordering: SSH bias ---

func TestCopyToClipboard_SSHPrefersOSC52(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env: map[string]string{
			"SSH_TTY": "/dev/pts/0",
			"DISPLAY": ":0", // would otherwise enable xclip
		},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
		runtimeOS:  "linux",
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			t.Errorf("native ladder must not run under SSH, got %q", c.name)
			return errors.New("must not be called")
		},
		lookPath: func(string) (string, error) { return "/usr/bin/x", nil },
	})

	method, err := copyToClipboard(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if method != copyMethodOSC52 {
		t.Errorf("expected OSC 52, got %v", method)
	}
	if buf.Len() == 0 {
		t.Errorf("expected OSC 52 emission")
	}
}

func TestCopyToClipboard_LocalPrefersNative(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env: map[string]string{
			"DISPLAY": ":0",
		},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
		runtimeOS:  "linux",
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			return nil // first attempt succeeds
		},
		lookPath: func(string) (string, error) { return "/usr/bin/x", nil },
	})

	method, err := copyToClipboard(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if method != copyMethodNative {
		t.Errorf("expected native, got %v", method)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no OSC 52 emission when native worked")
	}
}

func TestCopyToClipboard_FallsBackToOSC52WhenNativeFails(t *testing.T) {
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env: map[string]string{
			"DISPLAY": ":0",
		},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
		runtimeOS:  "linux",
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			return errors.New("boom")
		},
		lookPath: func(string) (string, error) { return "/usr/bin/x", nil },
	})

	method, err := copyToClipboard(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if method != copyMethodOSC52 {
		t.Errorf("expected OSC 52 fallback, got %v", method)
	}
}

func TestCopyToClipboard_AllBackendsFail(t *testing.T) {
	withSeams(t, seamOverrides{
		env:        map[string]string{},
		osc52IsTTY: boolPtr(false),
		runtimeOS:  "linux",
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			return errors.New("boom")
		},
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
	})

	_, err := copyToClipboard(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error when no backend works")
	}
}

func TestCopyToClipboard_OversizeFallsBackToTruncatedOSC52(t *testing.T) {
	// SSH-like situation: no native tool works, OSC 52 is a TTY.
	// Oversized payload should produce a truncated OSC 52 success — better
	// than copying nothing.
	var buf bytes.Buffer
	withSeams(t, seamOverrides{
		env: map[string]string{
			"SSH_TTY": "/dev/pts/0",
		},
		osc52Out:   &buf,
		osc52IsTTY: boolPtr(true),
		runtimeOS:  "linux",
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			return errors.New("should not reach")
		},
		lookPath: func(string) (string, error) { return "", errors.New("nope") },
	})

	big := strings.Repeat("a", osc52SafeLimit+1024)
	method, err := copyToClipboard(context.Background(), big)
	if err != nil {
		t.Fatalf("expected success via truncated OSC 52, got %v", err)
	}
	if method != copyMethodOSC52 {
		t.Errorf("expected OSC 52 method, got %v", method)
	}
	if buf.Len() == 0 {
		t.Errorf("expected truncated emission, got nothing")
	}
}

func TestCopyToClipboard_OversizeFailsWhenNotTTY(t *testing.T) {
	// Without a TTY we can't even truncate-and-send via OSC 52.
	withSeams(t, seamOverrides{
		env:        map[string]string{},
		osc52IsTTY: boolPtr(false),
		runtimeOS:  "linux",
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			return errors.New("boom")
		},
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
	})

	big := strings.Repeat("a", osc52SafeLimit+1)
	_, err := copyToClipboard(context.Background(), big)
	if err == nil {
		t.Fatal("expected error when no TTY and no native tool")
	}
}

// --- WaitDelay handling: xclip/xsel daemonize, hold the original stdin pipe;
// exec.ErrWaitDelay is the foreground-exit signal and should count as success.

func TestRunNativeClipboard_WaitDelayCountsAsSuccess(t *testing.T) {
	withSeams(t, seamOverrides{
		nativeRun: func(ctx context.Context, c nativeCmd, text string) error {
			return exec.ErrWaitDelay
		},
		lookPath: func(string) (string, error) { return "/usr/bin/xclip", nil },
	})

	ok := tryNativeClipboard(context.Background(), []nativeCmd{
		{name: "xclip", args: []string{"-selection", "clipboard"}, stdin: true},
	}, "hello")

	if !ok {
		t.Errorf("expected ErrWaitDelay to count as success (daemonized child)")
	}
}

// --- helpers ---

func candNames(cs []nativeCmd) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.name
	}
	return out
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// decodeOSC52 extracts and base64-decodes the payload from a raw OSC 52
// sequence ("\x1b]52;c;<base64>\x07"). Fails the test on shape mismatch.
func decodeOSC52(t *testing.T, seq string) string {
	t.Helper()
	const prefix = "\x1b]52;c;"
	const suffix = "\x07"
	i := strings.Index(seq, prefix)
	j := strings.LastIndex(seq, suffix)
	if i < 0 || j <= i+len(prefix) {
		t.Fatalf("not a recognisable OSC 52 sequence: %q", seq)
	}
	enc := seq[i+len(prefix) : j]
	out, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("OSC 52 payload not valid base64: %v", err)
	}
	return string(out)
}

func utf8Valid(s string) bool { return utf8.ValidString(s) }
