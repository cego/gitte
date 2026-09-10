package cmd

import (
	"sort"
	"strings"

	"github.com/cego/gitte/startup"
	"github.com/cego/gitte/telemetry"

	"go.opentelemetry.io/otel/codes"

	"github.com/spf13/cobra"
)

func newStartupCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "startup [check...]",
		Long: "Run all startup checks, or the named checks and their prerequisites.",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			if globalCfg != nil {
				for name := range globalCfg.StartupChecks {
					if strings.HasPrefix(name, toComplete) {
						names = append(names, name)
					}
				}
			}
			sort.Strings(names)
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		Short: "Run startup checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, span := telemetry.StartPhaseSpan(globalCtx, "startup")
			defer span.End()
			err := startup.Run(ctx, globalCfg, globalCwd, outputMode(), args...)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return err
		},
	}
}
