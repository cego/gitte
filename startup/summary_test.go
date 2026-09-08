package startup

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cego/gitte/executor"
)

func TestSummary_BlockedRootCausesAndDiagnostics(t *testing.T) {
	s := newCheckSummary([]executor.Task{
		{Name: "daemon"}, {Name: "swarm", Needs: []string{"daemon"}}, {Name: "networks", Needs: []string{"swarm", "daemon"}}, {Name: "git"},
	})
	s.record("networks", fmt.Errorf("skipped: %w", executor.ErrTaskSkipped), 0)
	s.record("git", nil, time.Millisecond)
	s.record("daemon", &checkFailure{cause: errors.New("permission denied\ncheck socket ownership"), hint: "Start **Docker**.\n\n~~~sh\ndocker info\n~~~", markdown: true}, time.Millisecond)
	s.record("swarm", executor.ErrTaskSkipped, 0)
	for _, styled := range []bool{false, true} {
		text := cleanText(s.render(styled, 80))
		for _, want := range []string{"1 failed · 2 blocked · 1 passed", "FAILED daemon", "permission denied\n  check socket ownership", "How to fix", "docker info", "networks: waiting for daemon", "swarm: waiting for daemon"} {
			if !strings.Contains(text, want) {
				t.Errorf("summary missing %q:\n%s", want, text)
			}
		}
		if strings.Count(text, "How to fix") != 1 || strings.Contains(text, "FAILED networks") {
			t.Errorf("blocked checks shown as failures:\n%s", text)
		}
	}
}

func TestGuidanceRendering_ReadablePlainTextAndUnbrokenCommands(t *testing.T) {
	command := "some-tool --path '/a long path/with spaces' --argument 'more words'"
	text := "**Repair** the `configuration`.\n\n1. Run this command:\n\n~~~sh\n" + command + "\n~~~\n\n2. Retry."
	for _, styled := range []bool{false, true} {
		got := cleanText(renderGuidance(text, styled, 24, true))
		if !strings.Contains(got, "    "+command+"\n") {
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
