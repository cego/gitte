package actions

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ResetToLatest resets the repo at dir to the latest origin/<branch>.
// It runs: git fetch origin <branch> && git reset --hard origin/<branch>
// This does not require a clean working tree — ALL local changes will be destroyed.
func ResetToLatest(ctx context.Context, dir, branch string) error {
	fetch := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "origin", branch)
	if out, err := fetch.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	reset := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "origin/"+branch)
	if out, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset --hard failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GitCleanFdx runs git clean -fdx in dir, excluding each path in excludes.
// This permanently removes all untracked files and directories.
func GitCleanFdx(ctx context.Context, dir string, excludes []string) error {
	args := []string{"-C", dir, "clean", "-fdx"}
	for _, ex := range excludes {
		args = append(args, "--exclude="+ex)
	}
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
