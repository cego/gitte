package startup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cego/gitte/config"
)

func TestGuidance_ContextAndFallback(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("GUIDANCE_EXAMPLE", "inherited")
	t.Setenv("GITTE_CHECK_NAME", "stale")
	cause := errors.New("check failed")
	cases := []struct {
		name, shell, script string
		generated           bool
	}{
		{"generated", "sh", `printf '%s\n%s\n%s' "$PWD" "$GITTE_CHECK_NAME" "$GUIDANCE_EXAMPLE"`, true},
		{"no output", "sh", "true", false},
		{"whitespace", "sh", "printf '  \\n'", false},
		{"nonzero", "sh", "printf partial; exit 1", false},
		{"stderr only", "sh", "printf secret >&2", false},
		{"missing interpreter", filepath.Join(cwd, "missing-shell"), "echo output", false},
		{"missing shell", "", "echo output", false},
		{"missing script", "sh", "", false},
		{"excess output", "sh", "while :; do printf '0123456789012345678901234567890123456789'; done", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := &config.CommandStartupCheck{BaseStartupCheck: config.BaseStartupCheck{
				Hint: "fallback", Guidance: &config.StartupGuidance{Shell: tc.shell, Script: tc.script},
			}}
			got := describeFailure(context.Background(), check, "tool-present", cwd, cause)
			var failure *checkFailure
			if !errors.As(got, &failure) || !errors.Is(got, cause) {
				t.Fatalf("lost structured cause: %v", got)
			}
			if tc.generated {
				for _, want := range []string{cwd, "tool-present", "inherited"} {
					if !strings.Contains(failure.hint, want) {
						t.Errorf("guidance %q missing %q", failure.hint, want)
					}
				}
				if !failure.markdown {
					t.Error("generated output must be Markdown")
				}
			} else if failure.hint != "fallback" || failure.markdown {
				t.Fatalf("fallback = %#v", failure)
			}
		})
	}
}

func TestGuidance_TimeoutAndCancellation(t *testing.T) {
	cwd := t.TempDir()
	start := time.Now()
	_, err := generateGuidance(context.Background(), &config.StartupGuidance{Shell: "sh", Script: "sleep 30 & wait"}, "slow", cwd, 30*time.Millisecond)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("timeout: elapsed=%s err=%v", time.Since(start), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	check := &config.CommandStartupCheck{BaseStartupCheck: config.BaseStartupCheck{
		Hint: "fallback", Guidance: &config.StartupGuidance{Shell: "sh", Script: "touch should-not-exist"},
	}}
	got := describeFailure(ctx, check, "cancelled", cwd, context.Canceled)
	var failure *checkFailure
	if !errors.As(got, &failure) || failure.hint != "fallback" {
		t.Fatalf("cancelled fallback: %v", got)
	}
	if _, err := os.Stat(filepath.Join(cwd, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("cancelled guidance executed")
	}
}

func TestGuidance_BashAndOrConditions(t *testing.T) {
	script := `if [ "$EXAMPLE_OS" = Linux ] && { [ "$EXAMPLE_MANAGER" = brew ] || [ "$EXAMPLE_MANAGER" = apt ]; }; then
 printf '%s' "$EXAMPLE_MANAGER"
 else exit 1; fi`
	for _, tc := range []struct {
		os, manager string
		ok          bool
	}{{"Linux", "brew", true}, {"Linux", "apt", true}, {"Linux", "unknown", false}, {"Darwin", "apt", false}} {
		t.Run(tc.os+"-"+tc.manager, func(t *testing.T) {
			t.Setenv("EXAMPLE_OS", tc.os)
			t.Setenv("EXAMPLE_MANAGER", tc.manager)
			text, err := generateGuidance(context.Background(), &config.StartupGuidance{Shell: "bash", Script: script}, "example", t.TempDir(), guidanceTimeout)
			if (err == nil) != tc.ok || (tc.ok && text != tc.manager) {
				t.Fatalf("text=%q err=%v", text, err)
			}
		})
	}
}
