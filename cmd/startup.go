package cmd

import (
	"gitte/startup"

	"github.com/spf13/cobra"
)

func newStartupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "startup",
		Short: "Run startup checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return startup.Run(globalCtx, globalCfg, globalCwd, outputMode())
		},
	}
}
