# Clean Command Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flag-based reporting-only `gitte clean` with destructive subcommands matching 1.x behaviour.

**Architecture:** Single file rewrite of `cmd/clean.go`. The root `clean` command has no `RunE` (cobra shows help by default). Four subcommands — `untracked`, `local-changes`, `master`, `all` — each call an exported-enough helper function so the logic is testable. The `--non-gitte` flag and all reporting-only code are removed.

**Tech Stack:** Go, cobra (`github.com/spf13/cobra`), `executor.ExecuteSyncInDir`, `bufio.Scanner` for stdin prompts.

---

### Task 1: Rewrite cmd/clean.go

**Files:**
- Modify: `cmd/clean.go` (full rewrite)
- Modify: `README.md` (clean section)
- Modify: `docs/commands.md` (gitte clean section)

- [ ] **Step 1: Replace cmd/clean.go with the subcommand structure**

Replace the entire contents of `cmd/clean.go` with:

```go
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

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Cleanup repositories",
		Long: `Cleanup operations on project repositories.

Subcommands:
  untracked     Remove untracked files (git clean -fdx)
  local-changes Reset repos with local changes (git reset --hard)
  master        Checkout the default branch in all repos
  all           Run all of the above in sequence`,
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
			fmt.Println()
			if err := runCleanLocalChanges(globalCtx, globalRawCfg, globalCwd, os.Stdin); err != nil {
				return err
			}
			fmt.Println()
			return runCleanMaster(globalCtx, globalRawCfg, globalCwd)
		},
	}
}

// runCleanUntracked runs git clean -fdx in every configured project that exists on disk.
func runCleanUntracked(ctx context.Context, cfg *config.GitteConfig, cwd string) error {
	for _, name := range sortedProjectNames(cfg) {
		proj := cfg.Projects[name]
		projectPath, ok := resolveProjectPath(cwd, name, proj)
		if !ok {
			continue
		}
		res, err := executor.ExecuteSyncInDir(ctx, projectPath, "git", "clean", "-fdx")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: [%s] git clean: %v\n", name, err)
			continue
		}
		if res.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "warning: [%s] git clean failed (exit %d): %s\n",
				name, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
		}
	}
	return nil
}

// runCleanLocalChanges finds repos with local changes, prompts the user, then resets.
func runCleanLocalChanges(ctx context.Context, cfg *config.GitteConfig, cwd string, stdin io.Reader) error {
	type dirtyRepo struct {
		name string
		path string
	}
	var dirty []dirtyRepo

	for _, name := range sortedProjectNames(cfg) {
		proj := cfg.Projects[name]
		projectPath, ok := resolveProjectPath(cwd, name, proj)
		if !ok {
			continue
		}
		res, err := executor.ExecuteSyncInDir(ctx, projectPath, "git", "status", "--porcelain")
		if err != nil || res.ExitCode != 0 {
			continue
		}
		if len(strings.TrimSpace(string(res.Stdout))) > 0 {
			dirty = append(dirty, dirtyRepo{name: name, path: projectPath})
		}
	}

	if len(dirty) == 0 {
		fmt.Println("No repos with local changes.")
		return nil
	}

	fmt.Println("Repos with local changes:")
	for _, r := range dirty {
		fmt.Printf("  %s\n", r.name)
	}

	scanner := bufio.NewScanner(stdin)

	fmt.Print("\nReset all, handle individually, or cancel? [all/individually/cancel]: ")
	scanner.Scan()
	choice := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch choice {
	case "all":
		for _, r := range dirty {
			if err := hardReset(ctx, r.name, r.path); err != nil {
				fmt.Fprintf(os.Stderr, "warning: [%s] %v\n", r.name, err)
			}
		}
	case "individually":
		for _, r := range dirty {
			fmt.Printf("Reset %s? [y/N]: ", r.name)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "y" || answer == "yes" {
				if err := hardReset(ctx, r.name, r.path); err != nil {
					fmt.Fprintf(os.Stderr, "warning: [%s] %v\n", r.name, err)
				}
			}
		}
	default:
		fmt.Println("Cancelled.")
	}

	return nil
}

// runCleanMaster checks out the default branch in every configured project.
func runCleanMaster(ctx context.Context, cfg *config.GitteConfig, cwd string) error {
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
		res, err := executor.ExecuteSyncInDir(ctx, projectPath, "git", "checkout", branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: [%s] git checkout %s: %v\n", name, branch, err)
			continue
		}
		if res.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "warning: [%s] git checkout %s failed (exit %d): %s\n",
				name, branch, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
		}
	}
	return nil
}

// hardReset runs git reset --hard in dir, returning an error on failure.
func hardReset(ctx context.Context, name, dir string) error {
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
```

- [ ] **Step 2: Verify it builds**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 3: Update the clean section in README.md**

Find this block in `README.md`:

```markdown
### clean flags

Reports repos matching each condition — useful for spotting what needs attention.

```bash
gitte clean --untracked        # list repos with untracked files
gitte clean --local-changes    # list repos with local changes
gitte clean --master           # list repos on the default branch
gitte clean --non-gitte        # list directories not managed by gitte
```
```

Replace with:

```markdown
### clean subcommands

```bash
gitte clean untracked       # remove untracked files (git clean -fdx)
gitte clean local-changes   # reset repos with local changes (prompts first)
gitte clean master          # checkout the default branch in all repos
gitte clean all             # run all three in sequence
```
```

- [ ] **Step 4: Update the clean section in docs/commands.md**

Find this block in `docs/commands.md`:

```markdown
## gitte clean

Report repo state. Each flag prints repos matching that condition.

```bash
gitte clean --untracked       # list repos with untracked files
gitte clean --local-changes   # list repos with local changes
gitte clean --master          # list repos currently on the default branch
gitte clean --non-gitte       # list directories not managed by gitte
```

Multiple flags can be combined.
```

Replace with:

```markdown
## gitte clean

Destructive cleanup operations on project repositories.

```bash
gitte clean untracked       # run git clean -fdx in every repo
gitte clean local-changes   # run git reset --hard in repos with local changes (prompts first)
gitte clean master          # run git checkout <default_branch> in every repo
gitte clean all             # run untracked → local-changes → master in sequence
```

`gitte clean local-changes` shows all affected repos, then asks:
`Reset all, handle individually, or cancel? [all/individually/cancel]`

If `individually`, you are prompted per repo: `Reset <name>? [y/N]`

Operations run on all configured projects regardless of toggle state.
```

- [ ] **Step 5: Final build and test check**

```bash
go build ./... && go test ./...
```

Expected: build succeeds, all existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/clean.go README.md docs/commands.md
git commit -m "Rewrite gitte clean as subcommands matching 1.x destructive behaviour"
```
