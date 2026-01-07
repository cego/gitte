package cmd

import (
	"fmt"
	"gitte/config"
	"gitte/internal"

	"github.com/spf13/cobra"
)

// gitopsCmd represents the gitops command
var toggleCmd = &cobra.Command{
	Use: "toggle",
	Annotations: map[string]string{
		"need-config": "true",
	},
	Short: "GitOps refer to GitOperations and will ensure that the enabled projects are cloned and up to date if possible.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		cfg := config.ConfigFromContext(ctx)
		toggledProjects, err := internal.ReadToggledProjects(config.CwdFromContext(ctx))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(toggleCmd)
}
