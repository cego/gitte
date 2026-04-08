package cmd

import (
	"fmt"

	"github.com/cego/gitte/actions"
	"github.com/cego/gitte/config"

	"github.com/spf13/cobra"
)

var flagActionsFilter []string
var flagActionsExclude []string

func newActionsCmd() *cobra.Command {
	cmd := &cobra.Command{
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
	cmd.Flags().BoolVar(&flagNoNeeds, "no-needs", false, "disable dependency resolution between tasks (also: GITTE_NO_NEEDS=true)")
	cmd.Flags().StringArrayVar(&flagActionsFilter, "filter", nil, "only run actions for projects matching these glob patterns (can be repeated)")
	cmd.Flags().StringArrayVar(&flagActionsExclude, "exclude", nil, "exclude projects matching these glob patterns (can be repeated)")
	return cmd
}

func runActions(args []string) error {
	cfg, err := config.FilterProjectsByGlob(globalCfg, flagActionsFilter, flagActionsExclude)
	if err != nil {
		return err
	}

	actionStr, groupStr, projectStr := parseActionArgs(args)
	keys, actionOrder := actions.PlanActions(cfg, actionStr, projectStr, groupStr, withNeeds())

	if len(keys) == 0 {
		return fmt.Errorf("no matching actions found for %q (action=%s projects=%q group=%q)",
			args[0], actionStr, projectStr, groupStr)
	}

	return actions.RunActions(globalCtx, cfg, globalSt, globalCwd, outputMode(), keys, actionOrder, maxParallelization())
}

// parseActionArgs maps positional CLI args to (actionStr, groupStr, projectStr).
// Argument order: <action> [group] [projects]
func parseActionArgs(args []string) (actionStr, groupStr, projectStr string) {
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
