package cmd

import (
	"github.com/cego/gitte/startup"
	"github.com/cego/gitte/telemetry"

	"go.opentelemetry.io/otel/codes"

	"github.com/spf13/cobra"
)

func newStartupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "startup",
		Short: "Run startup checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, span := telemetry.StartPhaseSpan(globalCtx, "startup")
			defer span.End()
			err := startup.Run(ctx, globalCfg, globalCwd, outputMode())
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			return err
		},
	}
}
