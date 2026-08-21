package gitops

import (
	"context"
	"errors"
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
	commitGitTestFile(t, dir, "tracked.txt", "feature\n", "feature")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
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
	if got := runGitTest(t, dir, "rev-parse", "refs/gitte/backups/feature/"+originalHead); got != originalHead {
		t.Errorf("backup ref = %s, want %s", got, originalHead)
	}
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

func TestTryRebase_PreservesStagedAndUnstagedChanges(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "master.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")

	writeGitTestFile(t, dir, "tracked.txt", "staged\n")
	runGitTest(t, dir, "add", "tracked.txt")
	writeGitTestFile(t, dir, "tracked.txt", "unstaged\n")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err != nil || !ok {
		t.Fatalf("tryRebase() = %v, %v; want true, nil", ok, err)
	}
	if got := runGitTest(t, dir, "status", "--porcelain"); got != "MM tracked.txt" {
		t.Fatalf("status = %q, want MM tracked.txt", got)
	}
	if got := readGitTestFile(t, dir, "tracked.txt"); got != "unstaged\n" {
		t.Errorf("worktree content = %q, want unstaged content", got)
	}
	if got := runGitTest(t, dir, "diff", "--cached", "--", "tracked.txt"); !strings.Contains(got, "+staged") {
		t.Errorf("cached diff = %q, want staged change", got)
	}
	assertNoGitteStash(t, dir)
}

func TestTryRebase_TrackedChangeConflictRestoresExactState(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "branch.txt", "branch\n", "branch")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "tracked.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	writeGitTestFile(t, dir, "tracked.txt", "staged\n")
	runGitTest(t, dir, "add", "tracked.txt")
	writeGitTestFile(t, dir, "tracked.txt", "unstaged\n")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
	originalStatus := runGitTest(t, dir, "status", "--porcelain=v2")
	originalCached := runGitTest(t, dir, "diff", "--cached", "--binary")
	originalWorktree := runGitTest(t, dir, "diff", "--binary")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err == nil || !strings.Contains(err.Error(), "could not be reapplied") {
		t.Fatalf("tryRebase() error = %v, want safe tracked-change conflict", err)
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	if got := runGitTest(t, dir, "status", "--porcelain=v2"); got != originalStatus {
		t.Errorf("status = %q, want %q", got, originalStatus)
	}
	if got := runGitTest(t, dir, "diff", "--cached", "--binary"); got != originalCached {
		t.Errorf("cached diff changed")
	}
	if got := runGitTest(t, dir, "diff", "--binary"); got != originalWorktree {
		t.Errorf("worktree diff changed")
	}
	assertNoRebaseState(t, dir)
	assertNoGitteStash(t, dir)
}

func TestTryRebase_IgnoredCollisionIsRejected(t *testing.T) {
	dir := initGitTestRepo(t)
	commitGitTestFile(t, dir, ".gitignore", "ignored.txt\n", "ignore")
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	writeGitTestFile(t, dir, "ignored.txt", "committed\n")
	runGitTest(t, dir, "add", "-f", "ignored.txt")
	runGitTest(t, dir, "commit", "-m", "ignored collision")
	runGitTest(t, dir, "switch", "feature")
	// The file is ignored in the feature worktree but exists in the target tree.
	writeGitTestFile(t, dir, "ignored.txt", "local\n")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err == nil {
		t.Fatal("tryRebase() error = nil, want blocked result")
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("error = %v, want blocked error", err)
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	if got := readGitTestFile(t, dir, "ignored.txt"); got != "local\n" {
		t.Errorf("ignored file = %q, want local content", got)
	}
}

func TestTryRebase_LocalMergeCommitIsRejected(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "side")
	commitGitTestFile(t, dir, "side.txt", "side\n", "side")
	runGitTest(t, dir, "switch", "-c", "feature", "master")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	runGitTest(t, dir, "merge", "--no-ff", "side", "-m", "merge side")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err == nil || !strings.Contains(err.Error(), "merge commits") {
		t.Fatalf("tryRebase() error = %v, want local merge rejection", err)
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
}

func TestTryRebase_PreExistingOperationIsUntouched(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "tracked.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "tracked.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	runGitMayFailTest(t, dir, "merge", "master")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err == nil || !strings.Contains(err.Error(), "operation already in progress") {
		t.Fatalf("tryRebase() error = %v, want active-operation rejection", err)
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err != nil {
		t.Fatalf("MERGE_HEAD was touched: %v", err)
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	runGitTest(t, dir, "merge", "--abort")
}

func TestTryRebase_DirtySubmoduleIsRejected(t *testing.T) {
	submodule := initGitTestRepo(t)
	main := initGitTestRepo(t)
	runGitTest(t, main, "-c", "protocol.file.allow=always", "submodule", "add", submodule, "nested")
	runGitTest(t, main, "commit", "-m", "submodule")
	runGitTest(t, main, "switch", "-c", "feature")
	commitGitTestFile(t, main, "feature.txt", "feature\n", "feature")
	writeGitTestFile(t, filepath.Join(main, "nested"), "tracked.txt", "dirty\n")
	originalHead := runGitTest(t, main, "rev-parse", "HEAD")

	ok, err := tryRebase(context.Background(), main, "master")
	if err == nil || !strings.Contains(err.Error(), "dirty submodule") {
		t.Fatalf("tryRebase() error = %v, want dirty submodule rejection", err)
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, main, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
}

func TestTryRebase_CanceledBeforeMutationLeavesState(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, err := tryRebase(ctx, dir, "master")
	if err == nil {
		t.Fatal("tryRebase() error = nil, want cancellation")
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	assertNoRebaseState(t, dir)
}

func TestTryRebase_PreservesExistingUserStash(t *testing.T) {
	dir := initGitTestRepo(t)
	writeGitTestFile(t, dir, "user.txt", "user stash\n")
	runGitTest(t, dir, "add", "user.txt")
	runGitTest(t, dir, "stash", "push", "-m", "user stash")
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "master.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	writeGitTestFile(t, dir, "tracked.txt", "local\n")

	ok, err := tryRebase(context.Background(), dir, "master")
	if err != nil || !ok {
		t.Fatalf("tryRebase() = %v, %v; want true, nil", ok, err)
	}
	if got := runGitTest(t, dir, "stash", "list"); !strings.Contains(got, "user stash") || strings.Contains(got, "gitte automatic sync") {
		t.Errorf("stash list = %q, want only existing user stash", got)
	}
}

func TestTryRebase_DoesNotRunHooks(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "master.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	hook := filepath.Join(dir, ".git", "hooks", "pre-rebase")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write pre-rebase hook: %v", err)
	}

	ok, err := tryRebase(context.Background(), dir, "master")
	if err != nil || !ok {
		t.Fatalf("tryRebase() = %v, %v; want true, nil with hooks disabled", ok, err)
	}
}

func TestMergeFastForward_PreservesTrackedAndUntrackedChanges(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "branch", "update")
	runGitTest(t, dir, "switch", "update")
	commitGitTestFile(t, dir, "remote.txt", "remote\n", "remote")
	runGitTest(t, dir, "switch", "master")

	writeGitTestFile(t, dir, "tracked.txt", "staged\n")
	runGitTest(t, dir, "add", "tracked.txt")
	writeGitTestFile(t, dir, "tracked.txt", "unstaged\n")
	writeGitTestFile(t, dir, "scratch.txt", "keep\n")

	upToDate, err := mergeFastForward(context.Background(), dir, "update")
	if err != nil || upToDate {
		t.Fatalf("mergeFastForward() = %v, %v; want pulled success", upToDate, err)
	}
	if got := readGitTestFile(t, dir, "scratch.txt"); got != "keep\n" {
		t.Errorf("untracked file = %q, want keep content", got)
	}
	if got := readGitTestFile(t, dir, "tracked.txt"); got != "unstaged\n" {
		t.Errorf("tracked file = %q, want unstaged content", got)
	}
	if got := runGitTest(t, dir, "status", "--porcelain"); got != "MM tracked.txt\n?? scratch.txt" {
		t.Errorf("status = %q, want staged/unstaged plus untracked", got)
	}
	assertNoGitteStash(t, dir)
}

func TestMergeFastForward_IgnoredCollisionRestoresExactState(t *testing.T) {
	dir := initGitTestRepo(t)
	commitGitTestFile(t, dir, ".gitignore", "ignored.txt\n", "ignore")
	runGitTest(t, dir, "branch", "update")
	runGitTest(t, dir, "switch", "update")
	writeGitTestFile(t, dir, "ignored.txt", "committed\n")
	runGitTest(t, dir, "add", "-f", "ignored.txt")
	runGitTest(t, dir, "commit", "-m", "track ignored file")
	runGitTest(t, dir, "switch", "master")
	writeGitTestFile(t, dir, "ignored.txt", "local\n")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")

	_, err := mergeFastForward(context.Background(), dir, "update")
	if err == nil {
		t.Fatal("mergeFastForward() error = nil, want ignored-file collision")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	if got := readGitTestFile(t, dir, "ignored.txt"); got != "local\n" {
		t.Errorf("ignored file = %q, want local content", got)
	}
	assertNoGitteStash(t, dir)
}

func TestCheckoutBranch_TrackedChangesAreUntouched(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "branch", "other")
	writeGitTestFile(t, dir, "tracked.txt", "local\n")

	err := checkoutBranch(context.Background(), dir, "other")
	if err == nil || !strings.Contains(err.Error(), "tracked changes") {
		t.Fatalf("checkoutBranch() error = %v, want tracked-change rejection", err)
	}
	if got := runGitTest(t, dir, "branch", "--show-current"); got != "master" {
		t.Errorf("branch = %s, want master", got)
	}
	if got := readGitTestFile(t, dir, "tracked.txt"); got != "local\n" {
		t.Errorf("tracked file = %q, want local content", got)
	}
}

func TestPathCollides_CaseSensitivity(t *testing.T) {
	if pathCollides("build/output", "build", false) != true {
		t.Error("directory prefix should collide")
	}
	if pathCollides("Foo.txt", "foo.txt", false) {
		t.Error("case-sensitive paths should not collide")
	}
	if !pathCollides("Foo.txt", "foo.txt", true) {
		t.Error("case-insensitive paths should collide")
	}
}

func TestTryRebase_ActiveBisectIsUntouched(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "feature.txt", "feature\n", "feature")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, ".git", "BISECT_START"), []byte("master\n"), 0o644); err != nil {
		t.Fatalf("write BISECT_START: %v", err)
	}

	ok, err := tryRebase(context.Background(), dir, "master")
	if err == nil || !strings.Contains(err.Error(), "bisect") {
		t.Fatalf("tryRebase() error = %v, want active-bisect rejection", err)
	}
	if ok {
		t.Fatal("tryRebase() = true, want false")
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
}

func TestWithTrackedChanges_CanceledRebaseRestoresExactState(t *testing.T) {
	dir := initGitTestRepo(t)
	runGitTest(t, dir, "switch", "-c", "feature")
	commitGitTestFile(t, dir, "tracked.txt", "feature\n", "feature")
	runGitTest(t, dir, "switch", "master")
	commitGitTestFile(t, dir, "tracked.txt", "master\n", "master")
	runGitTest(t, dir, "switch", "feature")
	writeGitTestFile(t, dir, "local.txt", "staged\n")
	runGitTest(t, dir, "add", "local.txt")
	writeGitTestFile(t, dir, "tracked.txt", "unstaged\n")
	originalHead := runGitTest(t, dir, "rev-parse", "HEAD")
	originalStatus := runGitTest(t, dir, "status", "--porcelain=v2")
	originalCached := runGitTest(t, dir, "diff", "--cached", "--binary")
	originalWorktree := runGitTest(t, dir, "diff", "--binary")
	ctx, cancel := context.WithCancel(context.Background())

	err := withTrackedChanges(ctx, dir, func() error {
		_, rebaseErr := rebaseRaw(context.Background(), dir, "master")
		if rebaseErr == nil {
			t.Fatal("rebaseRaw() error = nil, want conflict")
		}
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("withTrackedChanges() error = %v, want context canceled", err)
	}
	if got := runGitTest(t, dir, "rev-parse", "HEAD"); got != originalHead {
		t.Errorf("HEAD = %s, want %s", got, originalHead)
	}
	if got := runGitTest(t, dir, "status", "--porcelain=v2"); got != originalStatus {
		t.Errorf("status = %q, want %q", got, originalStatus)
	}
	if got := runGitTest(t, dir, "diff", "--cached", "--binary"); got != originalCached {
		t.Error("cached diff changed")
	}
	if got := runGitTest(t, dir, "diff", "--binary"); got != originalWorktree {
		t.Error("worktree diff changed")
	}
	assertNoRebaseState(t, dir)
	assertNoGitteStash(t, dir)
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

func assertNoGitteStash(t *testing.T, dir string) {
	t.Helper()
	if got := runGitTest(t, dir, "stash", "list"); strings.Contains(got, "gitte automatic sync") {
		t.Errorf("temporary Gitte stash remains: %q", got)
	}
}

func runGitMayFailTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
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
