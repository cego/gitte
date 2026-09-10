package config

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStartupCheck_Diagnostics(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"stderr", "printf 'socket unavailable' >&2; exit 7", "exited with code 7: socket unavailable"},
		{"stdout", "printf 'no identities'; exit 1", "exited with code 1: no identities"},
		{"stderr preferred", "printf noise; printf useful >&2; exit 1", "exited with code 1: useful"},
		{"bounded", "i=0; while [ $i -lt 2000 ]; do printf '01234567890123456789' >&2; i=$((i+1)); done; printf tail >&2; exit 1", "tail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks := []StartupCheck{
				&CommandStartupCheck{Command: []string{"sh", "-c", tc.script}},
				&ShellStartupCheck{Shell: "sh", Script: tc.script},
			}
			for _, check := range checks {
				err := check.Check(context.Background(), t.TempDir(), io.Discard, io.Discard)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("diagnostic = %v, want %q", err, tc.want)
				}
				if len(err.Error()) > 17*1024 {
					t.Fatalf("unbounded diagnostic: %d", len(err.Error()))
				}
			}
		})
	}
}

func TestStartupGuidance_ParsingAndValidation(t *testing.T) {
	cfg, err := LoadGitteConfigFromYAML([]byte(`startup:
  example:
    type: command
    cmd: [tool, --version]
    hint: "fallback"
    guidance:
      shell: bash
      script: "printf instructions"
`))
	if err != nil {
		t.Fatal(err)
	}
	check := cfg.StartupChecks["example"]
	if check.GetHint() != "fallback" || check.GetGuidance().Shell != "bash" {
		t.Fatalf("check=%#v", check)
	}
	if result := ValidateConfig(cfg); result.HasErrors() {
		t.Fatalf("validation=%+v", result)
	}
	check.GetGuidance().Shell = ""
	check.GetGuidance().Script = " "
	if result := ValidateConfig(cfg); len(result.Errors) != 2 {
		t.Fatalf("validation=%+v", result)
	}
}

func TestStartupCheck_BackgroundProcess(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		cancel       bool
		want         string
	}{
		{"success", "sleep 1.2 & exit 0", false, ""},
		{"failure", "sleep 1.2 & printf diagnostic >&2; exit 7", false, "exited with code 7: diagnostic"},
		{"cancelled", "sleep 1.2 & wait", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks := []StartupCheck{
				&CommandStartupCheck{Command: []string{"sh", "-c", tc.script}},
				&ShellStartupCheck{Shell: "sh", Script: tc.script},
			}
			for _, check := range checks {
				ctx := context.Background()
				if tc.cancel {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 30*time.Millisecond)
					t.Cleanup(cancel)
				}
				err := check.Check(ctx, t.TempDir(), io.Discard, io.Discard)
				switch {
				case tc.cancel:
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Fatalf("cancelled check returned %v", err)
					}
				case tc.want != "":
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("check returned %v, want %q", err, tc.want)
					}
				case err != nil:
					t.Fatalf("successful background check returned %v", err)
				}
			}
		})
	}
}
