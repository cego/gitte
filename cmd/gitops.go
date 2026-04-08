package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/gitops"
	"github.com/cego/gitte/output"

	"github.com/spf13/cobra"
)

func newGitopsCmd() *cobra.Command {
	var discover bool
	var noRebase bool
	var filter []string
	var exclude []string
	var retry int

	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "Sync git repositories",
		Long: `Clone or pull all configured projects.

With --discover, gitte first queries each configured source (GitLab groups,
GitHub orgs) via their API, then clones or pulls every repository found.
Discovered repos are cloned alongside configured repos; overlapping paths are
handled naturally since both use the same local directory layout.

Discovery sources are stored locally in .gitte-override.yml so they do not
interfere with the shared .gitte.yml. Manage them with 'gitte sources'.

API tokens:
  GitLab  GITLAB_TOKEN  (read_api scope)
  GitHub  GITHUB_TOKEN  (read:org scope for private orgs)

The token env var name can be customised per source with 'gitte sources add
--token-env'. Without a token, only public groups/orgs are accessible.

SSH concurrency:
  Discovery clone/pull runs at most 8 SSH connections in parallel to avoid
  overwhelming the server. Override with GITTE_MAX_TASK_PARALLELIZATION=N.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := outputMode()
			warnings, addWarning := newWarnCollector()

			cfg, err := config.FilterProjectsByGlob(globalCfg, filter, exclude)
			if err != nil {
				return err
			}

			if discover {
				if err := gitops.Discover(globalCtx, cfg, globalCwd, mode, addWarning); err != nil {
					gitops.PrintWarnings(mode, warnings())
					return err
				}
			}
			nr := noRebase || os.Getenv("GITTE_NO_REBASE") == "true"
			err = gitops.Sync(globalCtx, cfg, globalCwd, mode, nr, retry, makePromptFn(mode), addWarning)
			gitops.PrintWarnings(mode, warnings())
			return err
		},
	}

	cmd.Flags().BoolVar(&discover, "discover", false, "also discover and sync repos from configured sources")
	cmd.Flags().BoolVar(&noRebase, "no-rebase", false, "skip auto-rebase onto default branch (also: GITTE_NO_REBASE=true)")
	cmd.Flags().StringArrayVar(&filter, "filter", nil, "only sync projects matching these glob patterns (can be repeated)")
	cmd.Flags().StringArrayVar(&exclude, "exclude", nil, "exclude projects matching these glob patterns (can be repeated)")
	cmd.Flags().IntVar(&retry, "retry", 0, "number of automatic retries for transient errors (SSH timeouts, etc.)")
	return cmd
}

// newWarnCollector returns a thread-safe warning collector pair: a getter that
// returns all accumulated warnings, and an adder to pass as warnFn.
func newWarnCollector() (get func() []string, add func(string)) {
	var mu sync.Mutex
	var ws []string
	get = func() []string {
		mu.Lock()
		defer mu.Unlock()
		return ws
	}
	add = func(msg string) {
		mu.Lock()
		ws = append(ws, msg)
		mu.Unlock()
	}
	return
}

// makePromptFn returns a function that asks the user whether to checkout the
// default branch for a project that needs attention (detached HEAD or stale).
// In plain mode it returns an error with the absolute project path and
// recommended git command so the caller can surface a structured failure.
func makePromptFn(mode output.OutputMode) func(gitops.CheckoutPrompt) (bool, error) {
	if mode == output.ModePlain {
		return func(p gitops.CheckoutPrompt) (bool, error) {
			return false, fmt.Errorf("[gitops:%s] %s\n  path: %s\n  recommendation: %s",
				p.ProjectName, p.Reason, p.ProjectPath, p.Recommendation)
		}
	}
	return func(p gitops.CheckoutPrompt) (bool, error) {
		fmt.Printf("\n  %s: %s\n  → %s\n  Checkout %s? [y/N] ",
			p.ProjectName, p.Reason, p.Recommendation, p.DefaultBranch)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y"), nil
		}
		return false, nil
	}
}
