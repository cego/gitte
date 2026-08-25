package cmd

import (
	"fmt"
	"os"

	"github.com/cego/gitte/gitops"
	"github.com/cego/gitte/startup"
	"github.com/cego/gitte/telemetry"
	"go.opentelemetry.io/otel/codes"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var discover bool
	var noRebase bool

	cmd := &cobra.Command{
		Use:   "run [action] [group] [projects]",
		Short: "Full pipeline: startup checks + git sync + actions",
		Long: `Run the full pipeline: startup checks, git sync, then actions.

Examples:
  gitte run start
  gitte run start local
  gitte run start+test local
  gitte run start local frontend+backend`,
		Args:              cobra.RangeArgs(0, 3),
		ValidArgsFunction: actionArgsCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Step 1: Startup checks
			startupCtx, startupSpan := telemetry.StartPhaseSpan(globalCtx, "startup")
			serr := startup.Run(startupCtx, globalCfg, globalCwd, outputMode())
			if serr != nil {
				startupSpan.RecordError(serr)
				startupSpan.SetStatus(codes.Error, serr.Error())
			}
			startupSpan.End()
			if serr != nil {
				return serr
			}

			fmt.Println()

			// Step 2: Discovery + git sync
			mode := outputMode()
			warnings, addWarning := newWarnCollector()
			gitopsCtx, gitopsSpan := telemetry.StartPhaseSpan(globalCtx, "gitops")
			gerr := func() error {
				if discover {
					if err := gitops.Discover(gitopsCtx, globalCfg, globalCwd, mode, addWarning); err != nil {
						gitops.PrintWarnings(mode, warnings())
						return err
					}
				}
				nr := noRebase || os.Getenv("GITTE_NO_REBASE") == "true"
				if err := gitops.Sync(gitopsCtx, globalCfg, globalCwd, mode, nr, makePromptFn(mode), addWarning); err != nil {
					gitops.PrintWarnings(mode, warnings())
					return err
				}
				gitops.PrintWarnings(mode, warnings())
				return nil
			}()
			if gerr != nil {
				gitopsSpan.RecordError(gerr)
				gitopsSpan.SetStatus(codes.Error, gerr.Error())
			}
			gitopsSpan.End()
			if gerr != nil {
				return gerr
			}

			fmt.Println()

			// Step 3: Actions (if specified) — runActions opens its own "actions" phase span.
			if len(args) > 0 {
				return runActions(args)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&discover, "discover", false, "discover and sync repos from configured sources before actions")
	cmd.Flags().BoolVar(&noRebase, "no-rebase", false, "skip auto-rebase onto default branch (also: GITTE_NO_REBASE=true)")
	cmd.Flags().BoolVar(&flagNoNeeds, "no-needs", false, "disable dependency resolution between tasks (also: GITTE_NO_NEEDS=true)")
	return cmd
}
