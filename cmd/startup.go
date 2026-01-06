package cmd

import (
	"gitte/config"
	"gitte/internal"

	"github.com/spf13/cobra"
)

// startupCmd represents the startup command
var startupCmd = &cobra.Command{
	Use: "startup",
	Annotations: map[string]string{
		"need-config": "true",
	},
	Short: "Startup ensures that all dependencies are in place before running other commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		return internal.PerformStartupChecks(ctx, config.CwdFromContext(ctx), *config.ConfigFromContext(ctx))
	},
}

func init() {
	rootCmd.AddCommand(startupCmd)
}
