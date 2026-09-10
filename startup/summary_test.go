package startup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cego/gitte/executor"
)

func TestSummary_BlockedCountAndFailureDiagnostics(t *testing.T) {
	s := newCheckSummary()
	s.record("networks", fmt.Errorf("skipped: %w", executor.ErrTaskSkipped), 0)
	s.record("git", nil, time.Millisecond)
	s.record("daemon", &checkFailure{cause: errors.New("permission denied\ncheck socket ownership"), hint: "Start **Docker**.\n\n~~~sh\ndocker info\n~~~", markdown: true}, time.Millisecond)
	s.record("swarm", executor.ErrTaskSkipped, 0)
	for _, styled := range []bool{false, true} {
		text := cleanText(s.render(styled, 80))
		counts := "1 failed, 2 blocked, 1 passed"
		if styled {
			counts = "1 failed · 2 blocked · 1 passed"
		}
		if !strings.Contains(text, counts) {
			t.Errorf("summary missing %q: %s", counts, text)
		}
		for _, want := range []string{"FAILED daemon", "permission denied\n  check socket ownership", "How to fix", "docker info"} {
			if !strings.Contains(text, want) {
				t.Errorf("summary missing %q:\n%s", want, text)
			}
		}
		if strings.Count(text, "How to fix") != 1 || strings.Contains(text, "networks") || strings.Contains(text, "swarm") || strings.Contains(text, "Blocked by failed prerequisites") {
			t.Errorf("blocked check details shown in summary:\n%s", text)
		}
	}
}

func TestGuidanceRendering_ReadablePlainTextAndUnbrokenCommands(t *testing.T) {
	command := "some-tool --path '/a long path/with spaces' --argument 'more words'"
	text := "**Repair** the `configuration`.\n\n1. Run this command:\n\n~~~sh\n" + command + "\n~~~\n\n2. Retry."
	for _, styled := range []bool{false, true} {
		got := cleanText(renderGuidance(text, styled, 24, true))
		if !strings.Contains(got, "\n"+command+"\n") {
			t.Errorf("command was changed: %q", got)
		}
		for _, want := range []string{"Repair the", "configuration.", "1. Run this command:", "2. Retry."} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in %q", want, got)
			}
		}
		if strings.ContainsAny(got, "`~*") {
			t.Errorf("unrendered markup: %q", got)
		}
	}
}

func TestGuidanceRendering_FallbackIndentationAndControlSequences(t *testing.T) {
	got := renderGuidance("Run:\n  tool --arg value\n\x1b[31mvisible\x1b[0m\x1b]0;injected title\a", false, 80, false)
	if !strings.Contains(got, "      tool --arg value\n") || strings.Contains(got, "\x1b") || strings.Contains(got, "injected title") {
		t.Fatalf("unsafe or malformed output: %q", got)
	}
}

func TestSummary_SuccessHasNoSummary(t *testing.T) {
	for _, styled := range []bool{false, true} {
		s := newCheckSummary()
		if got := s.render(styled, 80); got != "" {
			t.Fatalf("empty run summary = %q", got)
		}
		s.record("tool", nil, time.Millisecond)
		if got := s.render(styled, 80); got != "" {
			t.Fatalf("successful run summary = %q", got)
		}
	}
}

func TestGuidanceRendering_NestedLists(t *testing.T) {
	text := "- parent\n  - **child**\n    1. `nested` item"
	for _, styled := range []bool{false, true} {
		got := cleanText(renderGuidance(text, styled, 80, true))
		want := "  - parent\n    - child\n      1. nested item\n"
		if got != want {
			t.Errorf("nested list = %q, want %q", got, want)
		}
	}
}

func TestGuidanceRendering_ClosingFences(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"longer closer", "```sh\ncommand\n`````\n**after**", "command\n  after\n"},
		{"longer opener", "````sh\n```\ncommand\n````\n**after**", "```\ncommand\n  after\n"},
		{"tilde fence", "~~~~sh\n~~~\ncommand\n~~~~~  \n**after**", "~~~\ncommand\n  after\n"},
		{"different marker", "```sh\n~~~\ncommand\n```\n**after**", "~~~\ncommand\n  after\n"},
		{"trailing text", "```sh\n``` trailing\ncommand\n```\n**after**", "``` trailing\ncommand\n  after\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, styled := range []bool{false, true} {
				if got := cleanText(renderGuidance(tc.input, styled, 80, true)); got != tc.want {
					t.Errorf("rendered = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestGuidanceRendering_CopyableHeredocs(t *testing.T) {
	for _, tc := range []struct {
		name, script, want string
	}{
		{"literal whitespace", "cat > result <<'EOF'\n\tliteral tab\n\n  spaces stay  \nEOF\nprintf appended >> result\n", "\tliteral tab\n\n  spaces stay  \nappended"},
		{"tab-stripping heredoc", "cat > result <<-'EOF'\n\tcontent\n\tEOF\nprintf appended >> result\n", "content\nappended"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, styled := range []bool{false, true} {
				text := renderGuidance("```sh\n"+tc.script+"```", styled, 20, true)
				script := stripEscapes(text)
				if script != tc.script {
					t.Fatalf("styled=%t: code whitespace changed: got %q, want %q", styled, script, tc.script)
				}
				cwd := t.TempDir()
				cmd := exec.CommandContext(context.Background(), "sh", "-c", script)
				cmd.Dir = cwd
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("rendered heredoc failed: %v\n%s", err, output)
				}
				data, err := os.ReadFile(filepath.Join(cwd, "result"))
				if err != nil || string(data) != tc.want {
					t.Fatalf("heredoc result = %q, error = %v, want %q", data, err, tc.want)
				}
			}
		})
	}
}

func TestGuidanceRendering_WrappedNestedLists(t *testing.T) {
	for _, tc := range []struct {
		name, text, want string
	}{
		{"bullet", "  - alpha beta gamma delta epsilon", "    - alpha beta gamma\n      delta epsilon\n"},
		{"emphasis", "  - alpha **beta** gamma `delta` epsilon", "    - alpha beta gamma\n      delta epsilon\n"},
		{"number", "    12) alpha beta gamma delta epsilon", "      12) alpha beta gamma\n          delta epsilon\n"},
		{"paragraph", "    alpha beta gamma delta epsilon", "      alpha beta gamma\n      delta epsilon\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, styled := range []bool{false, true} {
				if got := stripEscapes(renderGuidance(tc.text, styled, 24, true)); got != tc.want {
					t.Errorf("rendered = %q, want %q", got, tc.want)
				}
			}
		})
	}
}
