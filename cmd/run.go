package cmd

import (
	"gitte/gitops"
	"gitte/startup"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var discover bool

	cmd := &cobra.Command{
		Use:   "run [action] [projects] [group]",
		Short: "Full pipeline: startup checks + git sync + actions",
		Long: `Run the full pipeline: startup checks, git sync, then actions.

Examples:
  gitte run up
  gitte run up frontend+backend prod
  gitte run up * prod
  gitte run up+build`,
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
			if err := gitops.Sync(globalCtx, globalCfg, globalCwd); err != nil {
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
	return cmd
}
