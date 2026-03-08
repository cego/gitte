package gitops

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Sync clones or pulls all projects in the config using the executor
func Sync(ctx context.Context, cfg *config.GitteConfig, cwd string) error {
	tasks := make([]executor.Task, 0, len(cfg.Projects))

	for name, proj := range cfg.Projects {
		name := name
		proj := proj
		tasks = append(tasks, executor.Task{
			Name: fmt.Sprintf("gitops:%s", name),
			ExecuteFn: func(ctx context.Context, taskName string, handler executor.OutputHandler) error {
				return syncProject(ctx, cwd, name, proj, handler)
			},
		})
	}

	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{})
	if err != nil {
		return err
	}
	return exec.Execute(ctx)
}

// SyncTransient clones or pulls a list of transient (discovered) projects
func SyncTransient(ctx context.Context, remote, cwd string) error {
	localDir, err := config.LocalDirForRemote(remote)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, localDir)

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return cloneRemote(cwd, remote, localDir)
	}

	localChanges, err := hasLocalChanges(projectPath)
	if err != nil {
		return err
	}
	if localChanges {
		fmt.Printf("warning: skipping pull for %s (local changes present)\n", localDir)
		return nil
	}

	return pullFastForward(projectPath)
}

func syncProject(ctx context.Context, cwd, name string, proj config.ProjectConfig, handler executor.OutputHandler) error {
	localDir, err := config.LocalDirForRemote(proj.Remote)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, localDir)

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		_ = handler.HandleOutput(ctx, executor.Output{
			CmdName: "gitops:" + name,
			Stream:  executor.StdoutStream,
			Output:  []byte("Cloning...\n"),
		})
		return cloneRemote(cwd, proj.Remote, localDir)
	}

	localChanges, err := hasLocalChanges(projectPath)
	if err != nil {
		return err
	}
	if localChanges {
		_ = handler.HandleOutput(ctx, executor.Output{
			CmdName: "gitops:" + name,
			Stream:  executor.StdoutStream,
			Output:  []byte("warning: skipping pull (local changes present)\n"),
		})
		return nil
	}

	_ = handler.HandleOutput(ctx, executor.Output{
		CmdName: "gitops:" + name,
		Stream:  executor.StdoutStream,
		Output:  []byte("Pulling...\n"),
	})
	return pullFastForward(projectPath)
}

func cloneRemote(cwd, remote, localDir string) error {
	res, err := executor.ExecuteSyncInDir(cwd, "git", "clone", remote, localDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	if res.ExitCode != 0 {
		if regexp.MustCompile(`(?i)permission denied`).Match(res.Stderr) {
			return fmt.Errorf("permission denied cloning %q: check SSH keys", remote)
		}
		return fmt.Errorf("git clone failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

func pullFastForward(dir string) error {
	res, err := executor.ExecuteSyncInDir(dir, "git", "pull", "--ff-only")
	if err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	if res.ExitCode != 0 {
		stderr := string(res.Stderr)
		switch {
		case regexp.MustCompile(`(?i)no tracking information`).MatchString(stderr):
			return fmt.Errorf("no remote tracking branch in %s", dir)
		case regexp.MustCompile(`(?i)can't be fast-forwarded`).MatchString(stderr):
			return fmt.Errorf("branch cannot be fast-forwarded in %s", dir)
		case regexp.MustCompile(`(?i)conflicts with`).MatchString(stderr):
			return fmt.Errorf("conflicts with remote in %s", dir)
		default:
			return fmt.Errorf("git pull failed (exit %d): %s", res.ExitCode, strings.TrimSpace(stderr))
		}
	}
	return nil
}

func hasLocalChanges(dir string) (bool, error) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, fmt.Errorf("git status failed (exit %d)", res.ExitCode)
	}
	return len(strings.TrimSpace(string(res.Stdout))) > 0, nil
}
