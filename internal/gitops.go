package internal

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
	"os"
	"path/filepath"
	"regexp"
)

func GitOps(ctx context.Context, cwd string, gitteConfig config.GitteConfig) error {
	tasks := []executor.Task{}
	for name, pc := range gitteConfig.Projects {
		tasks = append(tasks, executor.Task{
			Name: fmt.Sprintf("gitops-%s", name),
			ExecuteFn: func(ctx context.Context, name string, oh executor.OutputHandler) error {
				return gitopsProject(ctx, cwd, pc)
			},
		})
	}

	return executor.NewExecutor(tasks).Execute(ctx)
}

func gitopsProject(ctx context.Context, cwd string, pc config.ProjectConfig) error {
	relativeDirectory, err := getProjectDirFromRemote(pc)
	if err != nil {
		return err
	}

	projectPath := filepath.Join(cwd, relativeDirectory)

	// Check if directory exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		// Directory does not exist, clone the repository
		fmt.Println("Cloning repository:", pc.Remote, "into", projectPath)
		return clone(ctx, cwd, pc)
	}

	localChanges, err := hasLocalChanges(projectPath)
	if err != nil {
		return err
	}

	if localChanges {
		return fmt.Errorf("local changes detected in project at '%s'. Please commit or stash them before pulling updates", projectPath)
	}

	err = pull(ctx, projectPath)
	if err != nil {
		return err
	}

	return nil // TODO log how far behind current branch is from default branch
}

func clone(ctx context.Context, cwd string, pc config.ProjectConfig) error {
	relativeDirectory, err := getProjectDirFromRemote(pc)
	if err != nil {
		return err
	}

	res, err := executor.ExecuteSyncInDir(cwd, "git", "clone", pc.Remote, relativeDirectory)
	if err != nil {
		return fmt.Errorf("failed to execute git clone command: %w", err)
	}

	if res.ExitCode != 0 {
		// If res.Stderr contains "Permission denied" return a specific error
		if regexp.MustCompile(`(?i)permission denied`).Match(res.Stderr) {
			return fmt.Errorf("permission denied when trying to clone repository '%s'. Please check your SSH keys and access rights", pc.Remote)
		}
		return fmt.Errorf("git clone command failed with exit code %d: \n%s", res.ExitCode, string(res.Stderr))
	}

	return nil
}

func getCurrentBranch(cwd string) (string, error) {
	res, err := executor.ExecuteSyncInDir(cwd, "git", "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to execute git branch command: %w", err)
	}

	if res.ExitCode != 0 {
		return "", fmt.Errorf("git branch command failed with exit code %d: \n%s", res.ExitCode, string(res.Stderr))
	}

	return string(res.Stdout), nil
}

func hasLocalChanges(cwd string) (bool, error) {
	res, err := executor.ExecuteSyncInDir(cwd, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to execute git status command: %w", err)
	}

	if res.ExitCode != 0 {
		return false, fmt.Errorf("git status command failed with exit code %d: \n%s", res.ExitCode, string(res.Stderr))
	}
	return len(res.Stdout) > 0, nil
}

func pull(ctx context.Context, cwd string) error {
	res, err := executor.ExecuteSyncInDir(cwd, "git", "pull", "--ff-only")
	if err != nil {
		return fmt.Errorf("failed to execute git pull command: %w", err)
	}

	if res.ExitCode != 0 {
		if regexp.MustCompile(`(?i)there is no tracking information for the current branch`).Match(res.Stderr) {
			return fmt.Errorf("the current branch doesn't have a remote origin in '%s'", cwd)
		} else if regexp.MustCompile(`(?i)your configuration specifies to merge with the ref`).Match(res.Stderr) {
			return fmt.Errorf("no such ref could be fetched in '%s'", cwd)
		} else if regexp.MustCompile(`(?i)conflicts with`).Match(res.Stderr) {
			return fmt.Errorf("the current branch has conflicts with the remote branch in '%s'", cwd)
		} else if regexp.MustCompile(`(?i)can't be fast-forwarded`).Match(res.Stderr) {
			return fmt.Errorf("the current branch cannot be fast-forwarded in '%s'", cwd)
		}

		return fmt.Errorf("git pull command failed with exit code %d: \n%s", res.ExitCode, string(res.Stderr))
	}

	return nil
}

func getProjectDirFromRemote(pc config.ProjectConfig) (string, error) {
	regex := regexp.MustCompile(`git@.*?:(.*)?\.git`)
	match := regex.FindStringSubmatch(pc.Remote)

	firstMatch := ""
	if len(match) > 1 {
		firstMatch = match[1]
	}

	if firstMatch == "" {
		return "", fmt.Errorf("'%' is not a valid git ssh url. Use git@gitlab.com:example/cego.git syntax", pc.Remote)
	}

	return firstMatch, nil
}

func getCommitCountBehindDefaultBranch(cwd string, branch string, defaultBranch string) (int, error) {
	res, err := executor.ExecuteSyncInDir(cwd, "git", "rev-list", "--count", "--left-right", fmt.Sprintf("%s...origin/%s", branch, defaultBranch))
	if err != nil {
		return 0, fmt.Errorf("failed to execute git rev-list command: %w", err)
	}

	if res.ExitCode != 0 {
		return 0, fmt.Errorf("git rev-list command failed with exit code %d: \n%s", res.ExitCode, string(res.Stderr))
	}

	var behindCount int
	_, err = fmt.Sscanf(string(res.Stdout), "%*d\t%d", &behindCount)
	if err != nil {
		return 0, fmt.Errorf("failed to parse git rev-list output: %w", err)
	}

	return behindCount, nil
}
