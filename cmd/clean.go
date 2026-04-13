package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Cleanup repositories",
	}
	cmd.AddCommand(
		newCleanUntrackedCmd(),
		newCleanLocalChangesCmd(),
		newCleanMasterCmd(),
		newCleanAllCmd(),
	)
	return cmd
}

func newCleanUntrackedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untracked",
		Short: "Remove untracked files from all repos (git clean -fdx)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanUntracked(globalCtx, globalRawCfg, globalCwd)
		},
	}
}

func newCleanLocalChangesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "local-changes",
		Short: "Reset repos with local changes (git reset --hard)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanLocalChanges(globalCtx, globalRawCfg, globalCwd, os.Stdin)
		},
	}
}

func newCleanMasterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "master",
		Short: "Checkout the default branch in all repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanMaster(globalCtx, globalRawCfg, globalCwd)
		},
	}
}

func newCleanAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "Run all cleanup operations: untracked, local-changes, master",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCleanUntracked(globalCtx, globalRawCfg, globalCwd); err != nil {
				return err
			}
			if err := runCleanLocalChanges(globalCtx, globalRawCfg, globalCwd, os.Stdin); err != nil {
				return err
			}
			return runCleanMaster(globalCtx, globalRawCfg, globalCwd)
		},
	}
}

// runCleanUntracked runs git clean -fdx concurrently across all configured
// projects that exist on disk, showing progress via cleanView.
func runCleanUntracked(ctx context.Context, cfg *config.GitteConfig, cwd string) error {
	type repoWork struct {
		name string
		path string
	}
	var repos []repoWork
	for _, name := range sortedProjectNames(cfg) {
		proj := cfg.Projects[name]
		projectPath, ok := resolveProjectPath(cwd, name, proj)
		if !ok {
			continue
		}
		repos = append(repos, repoWork{name, projectPath})
	}

	repoNames := make([]string, len(repos))
	for i, r := range repos {
		repoNames[i] = r.name
	}

	view := newCleanView(outputMode(), []cleanPhaseSpec{{Title: "Untracked", Repos: repoNames}})

	var wg sync.WaitGroup
	for _, r := range repos {
		wg.Add(1)
		go func(r repoWork) {
			defer wg.Done()
			view.OnStart("Untracked", r.name)
			res, err := executor.ExecuteSyncInDir(ctx, r.path, "git", "clean", "-fdx")
			if err != nil {
				view.OnFinish("Untracked", r.name, "", err)
				return
			}
			if res.ExitCode != 0 {
				view.OnFinish("Untracked", r.name, "",
					fmt.Errorf("exit %d: %s", res.ExitCode, strings.TrimSpace(string(res.Stderr))))
				return
			}
			detail := strings.TrimSpace(string(res.Stdout))
			if detail == "" {
				detail = "nothing to clean"
			}
			view.OnFinish("Untracked", r.name, detail, nil)
		}(r)
	}
	wg.Wait()
	view.Wait()
	return nil
}

// runCleanLocalChanges scans repos for local changes (with TUI), then
// prompts the user interactively (in plain text) and resets chosen repos.
func runCleanLocalChanges(ctx context.Context, cfg *config.GitteConfig, cwd string, stdin io.Reader) error {
	type repoWork struct {
		name string
		path string
	}
	type dirtyRepo struct {
		name string
		path string
	}

	var repos []repoWork
	for _, name := range sortedProjectNames(cfg) {
		proj := cfg.Projects[name]
		projectPath, ok := resolveProjectPath(cwd, name, proj)
		if !ok {
			continue
		}
		repos = append(repos, repoWork{name, projectPath})
	}

	repoNames := make([]string, len(repos))
	for i, r := range repos {
		repoNames[i] = r.name
	}

	view := newCleanView(outputMode(), []cleanPhaseSpec{{Title: "Local Changes", Repos: repoNames}})

	var mu sync.Mutex
	var dirty []dirtyRepo

	var wg sync.WaitGroup
	for _, r := range repos {
		wg.Add(1)
		go func(r repoWork) {
			defer wg.Done()
			view.OnStart("Local Changes", r.name)
			res, err := executor.ExecuteSyncInDir(ctx, r.path, "git", "status", "--porcelain")
			if err != nil || res.ExitCode != 0 {
				view.OnFinish("Local Changes", r.name, "error", nil)
				return
			}
			if len(strings.TrimSpace(string(res.Stdout))) > 0 {
				view.OnFinish("Local Changes", r.name, "has local changes", nil)
				mu.Lock()
				dirty = append(dirty, dirtyRepo{r.name, r.path})
				mu.Unlock()
			} else {
				view.OnFinish("Local Changes", r.name, "clean", nil)
			}
		}(r)
	}
	wg.Wait()
	view.Wait() // TUI exits here; prompt runs below in plain text

	if len(dirty) == 0 {
		fmt.Println("No repos with local changes.")
		return nil
	}

	fmt.Println("\nRepos with local changes:")
	for _, r := range dirty {
		fmt.Printf("  %s\n", r.name)
	}

	scanner := bufio.NewScanner(stdin)

	fmt.Print("\nReset all, handle individually, or cancel? [all/individually/cancel]: ")
	scanner.Scan()
	choice := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	switch choice {
	case "all":
		for _, r := range dirty {
			if err := hardReset(ctx, r.path); err != nil {
				fmt.Fprintf(os.Stderr, "warning: [%s] %v\n", r.name, err)
			}
		}
	case "individually":
		for _, r := range dirty {
			fmt.Printf("Reset %s? [y/N]: ", r.name)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "y" || answer == "yes" {
				if err := hardReset(ctx, r.path); err != nil {
					fmt.Fprintf(os.Stderr, "warning: [%s] %v\n", r.name, err)
				}
			}
		}
	default:
		fmt.Println("Cancelled.")
	}

	return nil
}

// runCleanMaster checks out the default branch concurrently across all
// configured projects, showing progress via cleanView.
func runCleanMaster(ctx context.Context, cfg *config.GitteConfig, cwd string) error {
	type repoWork struct {
		name   string
		path   string
		branch string
	}
	var repos []repoWork
	for _, name := range sortedProjectNames(cfg) {
		proj := cfg.Projects[name]
		projectPath, ok := resolveProjectPath(cwd, name, proj)
		if !ok {
			continue
		}
		branch := proj.DefaultBranch
		if branch == "" {
			branch = "master"
		}
		repos = append(repos, repoWork{name, projectPath, branch})
	}

	repoNames := make([]string, len(repos))
	for i, r := range repos {
		repoNames[i] = r.name
	}

	view := newCleanView(outputMode(), []cleanPhaseSpec{{Title: "Master", Repos: repoNames}})

	var wg sync.WaitGroup
	for _, r := range repos {
		wg.Add(1)
		go func(r repoWork) {
			defer wg.Done()
			view.OnStart("Master", r.name)
			res, err := executor.ExecuteSyncInDir(ctx, r.path, "git", "checkout", r.branch)
			if err != nil {
				view.OnFinish("Master", r.name, "", err)
				return
			}
			if res.ExitCode != 0 {
				view.OnFinish("Master", r.name, "",
					fmt.Errorf("exit %d: %s", res.ExitCode, strings.TrimSpace(string(res.Stderr))))
				return
			}
			view.OnFinish("Master", r.name, "→ "+r.branch, nil)
		}(r)
	}
	wg.Wait()
	view.Wait()
	return nil
}

// hardReset runs git reset --hard in dir, returning an error on failure.
func hardReset(ctx context.Context, dir string) error {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "reset", "--hard")
	if err != nil {
		return fmt.Errorf("git reset --hard: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git reset --hard failed (exit %d): %s",
			res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

// sortedProjectNames returns project names in alphabetical order.
func sortedProjectNames(cfg *config.GitteConfig) []string {
	names := make([]string, 0, len(cfg.Projects))
	for name := range cfg.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resolveProjectPath returns the absolute path to a project and whether it exists on disk.
// Logs a warning and returns false when the remote cannot be parsed or the dir does not exist.
func resolveProjectPath(cwd, name string, proj config.ProjectConfig) (string, bool) {
	localDir, err := config.LocalDirForRemote(proj.Remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot parse remote for %s: %v\n", name, err)
		return "", false
	}
	projectPath := filepath.Join(cwd, localDir)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return "", false
	}
	return projectPath, true
}
