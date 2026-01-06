package cmd

import (
	"gitte/config"
	"gitte/internal"

	"github.com/spf13/cobra"
)

// gitopsCmd represents the gitops command
var actionsCmd = &cobra.Command{
	Use: "actions <action> <group> [project]",
	Annotations: map[string]string{
		"need-config": "true",
	},
	Short: "GitOps refer to GitOperations and will ensure that the enabled projects are cloned and up to date if possible.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		return internal.GitOps(ctx, config.CwdFromContext(ctx), *config.ConfigFromContext(ctx))
	},
}

func init() {
	rootCmd.AddCommand(gitopsCmd)
}
