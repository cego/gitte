package internal

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
	"os"
	"regexp"
)

func GitOps(ctx context.Context, cwd string, gitteConfig config.GitteConfig) error {
	for _, pc := range gitteConfig.Projects {
		if err := gitopsProject(ctx, cwd, pc); err != nil {
			return err
		}
	}
	return nil
}

func gitopsProject(ctx context.Context, cwd string, pc config.ProjectConfig) error {
	relativeDirectory, err := getProjectDirFromRemote(pc)
	if err != nil {
		return err
	}

	projectPath := fmt.Sprintf("%s/%s", cwd, relativeDirectory) // TODO join prettier

	// Check if directory exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		// Directory does not exist, clone the repository
		fmt.Println("Cloning repository:", pc.Remote, "into", projectPath)
		return clone(ctx, cwd, pc)
	}
	return nil // TODO pull and update
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

func getProjectDirFromRemote(pc config.ProjectConfig) (string, error) {
	// 	const match = remote.match(/git@.*?:.*?\.git/);

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
