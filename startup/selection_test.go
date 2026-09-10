package startup

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/output"
)

func captureStartup(t *testing.T, cfg *config.GitteConfig, cwd string, names ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	result := make(chan string, 1)
	go func() { data, _ := io.ReadAll(r); _ = r.Close(); result <- string(data) }()
	runErr := Run(context.Background(), cfg, cwd, output.ModePlain, names...)
	_ = w.Close()
	return <-result, runErr
}

func TestStartup_SelectedChecksAndPrerequisites(t *testing.T) {
	cfg, err := config.LoadGitteConfigFromYAML([]byte(`startup:
  first:
    type: shell
    shell: sh
    script: "printf first >> order"
  selected:
    type: shell
    shell: sh
    needs: [first]
    script: "printf selected >> order"
    guidance:
      shell: sh
      script: "touch should-not-exist"
  unrelated:
    type: shell
    shell: sh
    script: "touch should-not-exist"
`))
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	text, err := captureStartup(t, cfg, cwd, "selected", "selected")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(cwd, "order"))
	if string(data) != "firstselected" {
		t.Fatalf("order=%q", data)
	}
	if _, err := os.Stat(filepath.Join(cwd, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("unrelated check or successful check guidance ran")
	}
	if strings.Contains(text, "Startup:") || !strings.Contains(text, "[startup:selected] OK") {
		t.Fatalf("summary=%s", text)
	}
}

func TestStartup_FailedAndBlockedGuidance(t *testing.T) {
	cfg, err := config.LoadGitteConfigFromYAML([]byte(`startup:
  first:
    type: shell
    shell: sh
    script: "printf diagnostic >&2; exit 1"
    hint: fallback
    guidance:
      shell: sh
      script: "printf '**Generated** instructions'"
  blocked:
    type: shell
    shell: sh
    needs: [first]
    script: "touch should-not-exist"
    guidance:
      shell: sh
      script: "touch should-not-exist"
`))
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	text, err := captureStartup(t, cfg, cwd)
	if err == nil {
		t.Fatal("expected failure")
	}
	for _, want := range []string{"diagnostic", "Generated instructions", "1 failed, 1 blocked, 0 passed", "[startup:blocked] BLOCKED"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(text, "fallback") {
		t.Error("fallback shown after successful guidance")
	}
	if _, err := os.Stat(filepath.Join(cwd, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("blocked check or guidance ran")
	}
}

func TestStartup_RejectsUnknownNamesAndCyclesBeforeRunning(t *testing.T) {
	cfg := &config.GitteConfig{StartupChecks: config.StartupCheckMap{
		"a": &config.ShellStartupCheck{BaseStartupCheck: config.BaseStartupCheck{Needs: []string{"b"}}, Shell: "sh", Script: "touch should-not-exist"},
		"b": &config.ShellStartupCheck{BaseStartupCheck: config.BaseStartupCheck{Needs: []string{"a"}}, Shell: "sh", Script: "touch should-not-exist"},
	}}
	for _, names := range [][]string{{"missing"}, {"a"}, nil} {
		cwd := t.TempDir()
		if err := Run(context.Background(), cfg, cwd, output.ModeTTY, names...); err == nil {
			t.Fatal("invalid selection accepted")
		}
		if _, err := os.Stat(filepath.Join(cwd, "should-not-exist")); !os.IsNotExist(err) {
			t.Fatal("invalid selection ran")
		}
	}
}

func TestStartup_UndefinedDependencyError(t *testing.T) {
	cfg := &config.GitteConfig{StartupChecks: config.StartupCheckMap{
		"selected":     &config.ShellStartupCheck{BaseStartupCheck: config.BaseStartupCheck{Needs: []string{"prerequisite"}}},
		"prerequisite": &config.ShellStartupCheck{BaseStartupCheck: config.BaseStartupCheck{Needs: []string{"missing"}}},
	}}
	for _, tc := range []struct {
		name, want string
	}{
		{"missing", `unknown startup check "missing"`},
		{"selected", `startup check "prerequisite" requires undefined check "missing"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(context.Background(), cfg, t.TempDir(), output.ModeTTY, tc.name)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Run() error = %v, want %s", err, tc.want)
			}
		})
	}
}
