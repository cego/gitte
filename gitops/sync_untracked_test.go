package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasTrackedChanges_OnlyTrackedFilesBlockSync(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testing.T, string)
		want   bool
	}{
		{
			name: "clean",
			change: func(t *testing.T, dir string) {
				t.Helper()
			},
		},
		{
			name: "untracked file",
			change: func(t *testing.T, dir string) {
				writeGitTestFile(t, dir, "untracked.txt", "untracked\n")
			},
		},
		{
			name: "modified tracked file",
			change: func(t *testing.T, dir string) {
				writeGitTestFile(t, dir, "tracked.txt", "modified\n")
			},
			want: true,
		},
		{
			name: "staged file",
			change: func(t *testing.T, dir string) {
				writeGitTestFile(t, dir, "staged.txt", "staged\n")
				runGitTest(t, dir, "add", "staged.txt")
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := initGitTestRepo(t)
			tc.change(t, dir)

			got, err := hasTrackedChanges(context.Background(), dir)
			if err != nil {
				t.Fatalf("hasTrackedChanges() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("hasTrackedChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTryRebase_UntrackedFileIsPreserved(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "master.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	writeGitTestFile(t, dir, "scratch.txt", "keep me\n")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err != nil {
		t.Fatalf("tryRebase() error = %v", err)
	}
	if !ok {
		t.Fatal("tryRebase() = false, want true")
	}
	if got := readGitTestFile(t, dir, "scratch.txt"); got != "keep me\n" {
		t.Errorf("untracked file = %q, want %q", got, "keep me\n")
	}
	runGitTest(t, dir, "merge-base", "--is-ancestor", "master", "HEAD")
	assertNoRebaseState(t, dir)
}

func TestTryRebase_UntrackedCollisionLeavesRepositoryUnchanged(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "collision.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	writeGitTestFile(t, dir, "collision.txt", "local\n")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err == nil {
		t.Fatal("tryRebase() error = nil, want untracked-file collision")
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	if got := readGitTestFile(t, dir, "collision.txt"); got != "local\n" {
		t.Errorf("untracked file = %q, want %q", got, "local\n")
	}
	assertNoRebaseState(t, dir)
}

func TestTryRebase_ConflictIsAborted(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "tracked.txt", "feature\n", "feature")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "tracked.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err != nil {
		t.Fatalf("tryRebase() error = %v", err)
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	if got := readGitTestFile(t, dir, "tracked.txt"); got != "feature\n" {
		t.Errorf("tracked file = %q, want %q", got, "feature\n")
	}
	assertNoRebaseState(t, dir)
}

func initGitTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitTest(t, dir, "init", "--initial-branch=master")
	runGitTest(t, dir, "config", "user.name", "Gitte Test")
	runGitTest(t, dir, "config", "user.email", "gitte@example.test")
	commitGitTestFile(t, dir, "tracked.txt", "initial\n", "initial")
	return dir
}

func commitGitTestFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	writeGitTestFile(t, dir, name, content)
	runGitTest(t, dir, "add", name)
	runGitTest(t, dir, "commit", "-m", message)
}

func writeGitTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readGitTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

func assertNoRebaseState(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"rebase-apply", "rebase-merge"} {
		if _, err := os.Stat(filepath.Join(dir, ".git", name)); !os.IsNotExist(err) {
			t.Errorf("unexpected %s state remains", name)
		}
	}
}

func runGitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
