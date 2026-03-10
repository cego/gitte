package cmd

import (
	"fmt"

	"gitte/actions"

	"github.com/spf13/cobra"
)

func newActionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "actions <action> [group] [projects]",
		Short: "Run actions only (no startup checks, no git sync)",
		Long: `Run actions only on configured projects.

Examples:
  gitte actions up
  gitte actions up sn
  gitte actions up+build sn
  gitte actions up sn evolution+promotion`,
		Args:              cobra.RangeArgs(1, 3),
		ValidArgsFunction: actionArgsCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActions(args)
		},
	}
}

func runActions(args []string) error {
	actionStr, projectStr, groupStr := parseActionArgs(args)
	keys, actionOrder := actions.PlanActions(globalCfg, actionStr, projectStr, groupStr, withNeeds())

	if len(keys) == 0 {
		return fmt.Errorf("no matching actions found for %q (action=%s projects=%q group=%q)",
			args[0], actionStr, projectStr, groupStr)
	}

	return actions.RunActions(globalCtx, globalCfg, globalSt, globalCwd, outputMode(), keys, actionOrder, maxParallelization())
}

// parseActionArgs maps positional CLI args to (actionStr, groupStr, projectStr).
// Argument order: <action> [group] [projects]
func parseActionArgs(args []string) (actionStr, projectStr, groupStr string) {
	if len(args) > 0 {
		actionStr = args[0]
	}
	if len(args) > 1 {
		groupStr = args[1]
	}
	if len(args) > 2 {
		projectStr = args[2]
	}
	return
}
