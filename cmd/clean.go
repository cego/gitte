package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gitte/config"
	"gitte/executor"

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
			return runClean(untracked, localChanges, master, nonGitte)
		},
	}

	cmd.Flags().BoolVar(&untracked, "untracked", false, "list repos with untracked files")
	cmd.Flags().BoolVar(&localChanges, "local-changes", false, "list repos with local changes")
	cmd.Flags().BoolVar(&master, "master", false, "list repos on master/main branch")
	cmd.Flags().BoolVar(&nonGitte, "non-gitte", false, "list directories not managed by gitte")

	return cmd
}

func runClean(untracked, localChanges, master, nonGitte bool) error {
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
			checkLocalState(name, projectPath, untracked, localChanges)
		}

		if master {
			checkBranch(name, projectPath)
		}
	}

	if nonGitte {
		checkNonGitte()
	}

	return nil
}

func checkLocalState(name, dir string, checkUntracked, checkLocalChanges bool) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "status", "--porcelain")
	if err != nil || res.ExitCode != 0 {
		return
	}

	output := string(res.Stdout)
	if checkUntracked {
		for _, line := range splitLines(output) {
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

func checkBranch(name, dir string) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "branch", "--show-current")
	if err != nil || res.ExitCode != 0 {
		return
	}
	branch := string(res.Stdout)
	branch = trimRight(branch, "\n\r")
	if branch == "master" || branch == "main" {
		fmt.Printf("[clean] %s: on default branch (%s)\n", name, branch)
	}
}

func checkNonGitte() {
	// Find all directories in cwd root and check against config
	gitteDirs := make(map[string]bool)
	for _, proj := range globalCfg.Projects {
		if dir, err := config.LocalDirForRemote(proj.Remote); err == nil {
			// Get top-level dir
			gitteDirs[filepath.Dir(dir)] = true
			gitteDirs[dir] = true
		}
	}
	fmt.Println("[clean] non-gitte check: not fully implemented yet")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimRight(s, cutset string) string {
	for len(s) > 0 {
		found := false
		for _, c := range cutset {
			if rune(s[len(s)-1]) == c {
				s = s[:len(s)-1]
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return s
}
