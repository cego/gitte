package cmd

import (
	"fmt"
	"gitte/output"
	"gitte/toggle"

	"github.com/spf13/cobra"
)

func newToggleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "toggle",
		Short: "Interactively enable/disable projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputMode() == output.ModePlain {
				return fmt.Errorf("toggle requires a TTY (cannot run in --no-tty mode)")
			}
			return toggle.Run(globalCfg, globalCwd, globalSt)
		},
	}
}
