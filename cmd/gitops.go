package cmd

import (
	"gitte/gitops"

	"github.com/spf13/cobra"
)

func newGitopsCmd() *cobra.Command {
	var discover bool

	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "Sync git repositories",
		Long:  "Clone or pull all configured projects. Use --discover to also fetch group/org repos.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if discover {
				if err := gitops.Discover(globalCtx, globalCfg, globalCwd); err != nil {
					return err
				}
			}
			return gitops.Sync(globalCtx, globalCfg, globalCwd)
		},
	}

	cmd.Flags().BoolVar(&discover, "discover", false, "also discover and sync repos from configured sources")
	return cmd
}
