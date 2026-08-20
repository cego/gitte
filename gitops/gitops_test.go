package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cego/gitte/config"
)

func TestSyncProject_DefaultBranchStatus(t *testing.T) {
	tests := []struct {
		name       string
		advance    int
		dirty      bool
		breakFetch bool
		wantDetail string
		wantPrompt bool
	}{
		{
			name:       "feature contains master",
			wantDetail: "up to date",
		},
		{
			name:       "feature misses recent master commits",
			advance:    2,
			wantDetail: "stale: behind master 2c/10d",
			wantPrompt: true,
		},
		{
			name:       "dirty feature misses recent master commits",
			advance:    1,
			dirty:      true,
			wantDetail: "stale: behind master 1c/10d +work",
			wantPrompt: true,
		},
		{
			name:       "fetch fails",
			breakFetch: true,
			wantDetail: "unknown: fetch failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newGitFixture(t)
			fixture.checkoutFeature(t)
			fixture.advanceMaster(t, tc.advance)
			if tc.dirty {
				writeTestFile(t, fixture.projectPath, "dirty.txt", "dirty\n")
			}
			if tc.breakFetch {
				runGit(t, fixture.projectPath, "remote", "set-url", "origin", filepath.Join(fixture.root, "missing.git"))
			}

			detail, prompts, warnings := fixture.sync(t, true)
			if detail != tc.wantDetail {
				t.Fatalf("detail = %q, want %q", detail, tc.wantDetail)
			}
			if got := len(prompts) > 0; got != tc.wantPrompt {
				t.Errorf("prompt shown = %v, want %v", got, tc.wantPrompt)
			}
			if tc.breakFetch && len(warnings) == 0 {
				t.Error("expected fetch warning")
			}
		})
	}
}

func TestSyncProject_AutoRebaseContainsDefaultBranch(t *testing.T) {
	fixture := newGitFixture(t)
	fixture.checkoutFeature(t)
	fixture.advanceMaster(t, 2)

	detail, prompts, _ := fixture.sync(t, false)
	if detail != "rebased onto master" {
		t.Fatalf("detail = %q, want %q", detail, "rebased onto master")
	}
	if len(prompts) != 0 {
		t.Fatalf("got %d prompts, want none", len(prompts))
	}
	runGit(t, fixture.projectPath, "merge-base", "--is-ancestor", "origin/master", "HEAD")
}

type gitFixture struct {
	root        string
	projectPath string
	seedPath    string
	remote      string
}

func newGitFixture(t *testing.T) gitFixture {
	t.Helper()
	root := t.TempDir()
	barePath := filepath.Join(root, "remote.git")
	seedPath := filepath.Join(root, "seed")
	projectPath := filepath.Join(root, "example.test", "org", "repo")

	runGit(t, root, "init", "--bare", "--initial-branch=master", barePath)
	runGit(t, root, "init", "--initial-branch=master", seedPath)
	configureTestGit(t, seedPath)
	writeTestFile(t, seedPath, "base.txt", "base\n")
	baseDate := time.Now().Add(-10*24*time.Hour - time.Hour).Format(time.RFC3339)
	runGitEnv(t, seedPath, []string{"GIT_AUTHOR_DATE=" + baseDate, "GIT_COMMITTER_DATE=" + baseDate}, "add", "base.txt")
	runGitEnv(t, seedPath, []string{"GIT_AUTHOR_DATE=" + baseDate, "GIT_COMMITTER_DATE=" + baseDate}, "commit", "-m", "base")
	runGit(t, seedPath, "remote", "add", "origin", barePath)
	runGit(t, seedPath, "push", "-u", "origin", "master")

	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("create project parent: %v", err)
	}
	runGit(t, root, "clone", barePath, projectPath)
	configureTestGit(t, projectPath)

	return gitFixture{
		root:        root,
		projectPath: projectPath,
		seedPath:    seedPath,
		remote:      "https://example.test/org/repo.git",
	}
}

func (f gitFixture) checkoutFeature(t *testing.T) {
	t.Helper()
	runGit(t, f.projectPath, "switch", "-c", "feature")
	writeTestFile(t, f.projectPath, "feature.txt", "feature\n")
	runGit(t, f.projectPath, "add", "feature.txt")
	runGit(t, f.projectPath, "commit", "-m", "feature")
}

func (f gitFixture) advanceMaster(t *testing.T, count int) {
	t.Helper()
	for i := range count {
		name := filepath.Join("master", time.Now().Add(time.Duration(i)*time.Second).Format("150405.000000000")+".txt")
		writeTestFile(t, f.seedPath, name, "master\n")
		runGit(t, f.seedPath, "add", name)
		runGit(t, f.seedPath, "commit", "-m", "advance master")
	}
	if count > 0 {
		runGit(t, f.seedPath, "push", "origin", "master")
	}
}

func (f gitFixture) sync(t *testing.T, noRebase bool) (string, []CheckoutPrompt, []string) {
	t.Helper()
	var detail string
	var prompts []CheckoutPrompt
	var warnings []string
	err := syncProject(
		context.Background(),
		f.root,
		"repo",
		config.ProjectConfig{Remote: f.remote, DefaultBranch: "master"},
		noRebase,
		func(value string) { detail = value },
		func(prompt CheckoutPrompt) { prompts = append(prompts, prompt) },
		func(warning string) { warnings = append(warnings, warning) },
	)
	if err != nil {
		t.Fatalf("syncProject() error = %v", err)
	}
	return detail, prompts, warnings
}

func configureTestGit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.name", "Gitte Test")
	runGit(t, dir, "config", "user.email", "gitte@example.test")
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGitEnv(t, dir, nil, args...)
}

func runGitEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
