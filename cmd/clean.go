package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"

	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var (
		untracked    bool
		localChanges bool
		master       bool
		nonGitte     bool
	)

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Cleanup repositories",
		Long: `Cleanup operations on project repositories.

Flags:
  --untracked      Show repos with untracked files
  --local-changes  Show repos with local changes
  --master         Show repos on master/main branch
  --non-gitte      Show dirs in root not in gitte config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(globalCtx, untracked, localChanges, master, nonGitte)
		},
	}

	cmd.Flags().BoolVar(&untracked, "untracked", false, "list repos with untracked files")
	cmd.Flags().BoolVar(&localChanges, "local-changes", false, "list repos with local changes")
	cmd.Flags().BoolVar(&master, "master", false, "list repos on master/main branch")
	cmd.Flags().BoolVar(&nonGitte, "non-gitte", false, "list directories not managed by gitte")

	return cmd
}

func runClean(ctx context.Context, untracked, localChanges, master, nonGitte bool) error {
	for name, proj := range globalCfg.Projects {
		localDir, err := config.LocalDirForRemote(proj.Remote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot parse remote for %s: %v\n", name, err)
			continue
		}

		projectPath := filepath.Join(globalCwd, localDir)

		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			continue
		}

		if untracked || localChanges {
			checkLocalState(ctx, name, projectPath, untracked, localChanges)
		}

		if master {
			checkBranch(ctx, name, projectPath)
		}
	}

	if nonGitte {
		checkNonGitte()
	}

	return nil
}

func checkLocalState(ctx context.Context, name, dir string, checkUntracked, checkLocalChanges bool) {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "status", "--porcelain")
	if err != nil || res.ExitCode != 0 {
		return
	}

	output := string(res.Stdout)
	if checkUntracked {
		for _, line := range strings.Split(output, "\n") {
			if len(line) >= 2 && line[0] == '?' && line[1] == '?' {
				fmt.Printf("[clean] %s: has untracked files\n", name)
				break
			}
		}
	}
	if checkLocalChanges && len(output) > 0 {
		fmt.Printf("[clean] %s: has local changes\n", name)
	}
}

func checkBranch(ctx context.Context, name, dir string) {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "branch", "--show-current")
	if err != nil || res.ExitCode != 0 {
		return
	}
	branch := strings.TrimRight(string(res.Stdout), "\n\r")
	if branch == "master" || branch == "main" {
		fmt.Printf("[clean] %s: on default branch (%s)\n", name, branch)
	}
}

func checkNonGitte() {
	gitteDirs := make(map[string]bool)
	for _, proj := range globalCfg.Projects {
		if dir, err := config.LocalDirForRemote(proj.Remote); err == nil {
			gitteDirs[dir] = true
		}
	}

	// Walk host/org/repo structure (3 levels deep) and report unknown dirs
	hostEntries, err := os.ReadDir(globalCwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[clean] cannot read cwd: %v\n", err)
		return
	}
	for _, hostEntry := range hostEntries {
		if !hostEntry.IsDir() {
			continue
		}
		orgEntries, err := os.ReadDir(filepath.Join(globalCwd, hostEntry.Name()))
		if err != nil {
			continue
		}
		for _, orgEntry := range orgEntries {
			if !orgEntry.IsDir() {
				continue
			}
			repoEntries, err := os.ReadDir(filepath.Join(globalCwd, hostEntry.Name(), orgEntry.Name()))
			if err != nil {
				continue
			}
			for _, repoEntry := range repoEntries {
				if !repoEntry.IsDir() {
					continue
				}
				relPath := filepath.Join(hostEntry.Name(), orgEntry.Name(), repoEntry.Name())
				if !gitteDirs[relPath] {
					fmt.Printf("[clean] non-gitte: %s\n", relPath)
				}
			}
		}
	}
}
