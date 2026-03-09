package cmd

import (
	"os"

	"gitte/gitops"
	"gitte/startup"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var discover bool
	var noRebase bool

	cmd := &cobra.Command{
		Use:   "run [action] [group] [projects]",
		Short: "Full pipeline: startup checks + git sync + actions",
		Long: `Run the full pipeline: startup checks, git sync, then actions.

Examples:
  gitte run up
  gitte run up sn
  gitte run up+build sn
  gitte run up sn evolution+promotion`,
		Args: cobra.RangeArgs(0, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Step 1: Startup checks
			if err := startup.Run(globalCtx, globalCfg, globalCwd, outputMode()); err != nil {
				return err
			}

			// Step 2: Discovery (if requested)
			if discover {
				if err := gitops.Discover(globalCtx, globalCfg, globalCwd); err != nil {
					return err
				}
			}

			// Step 3: Git sync
			mode := outputMode()
			nr := noRebase || os.Getenv("GITTE_NO_REBASE") == "true"
			if err := gitops.Sync(globalCtx, globalCfg, globalCwd, mode, nr, makePromptFn(mode)); err != nil {
				return err
			}

			// Step 4: Actions (if specified)
			if len(args) > 0 {
				return runActions(args)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&discover, "discover", false, "discover and sync repos from configured sources before actions")
	cmd.Flags().BoolVar(&noRebase, "no-rebase", false, "skip auto-rebase onto default branch (also: GITTE_NO_REBASE=true)")
	return cmd
}
