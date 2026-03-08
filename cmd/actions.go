package cmd

import (
	"fmt"
	"gitte/actions"

	"github.com/spf13/cobra"
)

func newActionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "actions [action] [projects] [group]",
		Short: "Run actions only (no startup checks, no git sync)",
		Long: `Run actions only on configured projects.

Examples:
  gitte actions up
  gitte actions up frontend+backend prod
  gitte actions up * prod
  gitte actions up+build`,
		Args: cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActions(args)
		},
	}
}

func runActions(args []string) error {
	actionStr, projectStr, groupStr := parseActionArgs(args)
	keys := actions.PlanActions(globalCfg, actionStr, projectStr, groupStr, withNeeds())

	if len(keys) == 0 {
		return fmt.Errorf("no matching actions found for %q", actionStr)
	}

	return actions.RunActions(globalCtx, globalCfg, globalSt, globalCwd, keys, maxParallelization())
}

func parseActionArgs(args []string) (actionStr, projectStr, groupStr string) {
	if len(args) > 0 {
		actionStr = args[0]
	}
	if len(args) > 1 {
		projectStr = args[1]
	}
	if len(args) > 2 {
		groupStr = args[2]
	}
	return
}
