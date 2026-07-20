package telemetry

import (
	"context"
	"sync"

	"github.com/cego/gitte/executor"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// logHandler forwards output to inner and emits a correlated OTEL log record.
type logHandler struct {
	inner executor.OutputHandler
	once  sync.Once
	lgr   log.Logger
}

// LogOutputHandler wraps inner so that every output line is also emitted as an
// OTEL log record correlated to the span in ctx. Callers should install this
// wrapper at the producer boundary, before output is handed to an asynchronous
// drain, so the task span is still available. When logs are disabled the global
// logger provider is a no-op; output is always forwarded unchanged.
//
// The logger is resolved lazily on first use so that callers constructed before
// telemetry.Init registers the real LoggerProvider still pick up the live
// provider.
func LogOutputHandler(inner executor.OutputHandler) executor.OutputHandler {
	return &logHandler{inner: inner}
}

func (h *logHandler) logger() log.Logger {
	h.once.Do(func() {
		h.lgr = global.GetLoggerProvider().Logger("github.com/cego/gitte")
	})
	return h.lgr
}

func (h *logHandler) HandleOutput(ctx context.Context, out executor.Output) error {
	h.emit(ctx, out)
	return h.inner.HandleOutput(ctx, out)
}

func (h *logHandler) emit(ctx context.Context, out executor.Output) {
	var rec log.Record
	rec.SetBody(log.StringValue(string(out.Output)))
	sev := log.SeverityInfo
	if out.Stream == executor.StderrStream {
		sev = log.SeverityWarn
	}
	rec.SetSeverity(sev)
	rec.AddAttributes(
		log.String("gitte.task", out.CmdName),
		log.String("stream", string(out.Stream)),
	)
	if len(out.Output) >= 6 && string(out.Output[:6]) == "[HINT]" {
		rec.AddAttributes(log.Bool("gitte.hint", true))
	}
	h.logger().Emit(ctx, rec)
}
