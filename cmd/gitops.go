package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cego/gitte/gitops"
	"github.com/cego/gitte/output"

	"github.com/spf13/cobra"
)

func newGitopsCmd() *cobra.Command {
	var discover bool
	var noRebase bool

	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "Sync git repositories",
		Long:  "Clone or pull all configured projects. Use --discover to also fetch group/org repos.",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := outputMode()
			if discover {
				if err := gitops.Discover(globalCtx, globalCfg, globalCwd, mode); err != nil {
					return err
				}
			}
			nr := noRebase || os.Getenv("GITTE_NO_REBASE") == "true"
			return gitops.Sync(globalCtx, globalCfg, globalCwd, mode, nr, makePromptFn(mode))
		},
	}

	cmd.Flags().BoolVar(&discover, "discover", false, "also discover and sync repos from configured sources")
	cmd.Flags().BoolVar(&noRebase, "no-rebase", false, "skip auto-rebase onto default branch (also: GITTE_NO_REBASE=true)")
	return cmd
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
