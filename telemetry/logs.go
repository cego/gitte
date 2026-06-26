package telemetry

import (
	"context"
	"sync"

	"github.com/cego/gitte/executor"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

// SpanRegistry maps a task name to the SpanContext of the span representing that
// task, so output lines (which arrive on a separate goroutine without the task
// span in their context) can be correlated to the right span.
type SpanRegistry struct {
	mu sync.RWMutex
	m  map[string]trace.SpanContext
}

// NewSpanRegistry returns an empty, concurrency-safe registry.
func NewSpanRegistry() *SpanRegistry {
	return &SpanRegistry{m: make(map[string]trace.SpanContext)}
}

func (r *SpanRegistry) Set(task string, sc trace.SpanContext) {
	r.mu.Lock()
	r.m[task] = sc
	r.mu.Unlock()
}

func (r *SpanRegistry) Get(task string) (trace.SpanContext, bool) {
	r.mu.RLock()
	sc, ok := r.m[task]
	r.mu.RUnlock()
	return sc, ok
}

func (r *SpanRegistry) Delete(task string) {
	r.mu.Lock()
	delete(r.m, task)
	r.mu.Unlock()
}

// logHandler forwards output to inner and emits a correlated OTEL log record.
type logHandler struct {
	inner executor.OutputHandler
	reg   *SpanRegistry
	lgr   log.Logger
}

// LogOutputHandler wraps inner so that every output line is also emitted as an
// OTEL log record correlated (via reg) to the span for output.CmdName. When
// logs are disabled the global logger provider is a no-op, so this is safe and
// cheap; output is always forwarded to inner unchanged.
func LogOutputHandler(inner executor.OutputHandler, reg *SpanRegistry) executor.OutputHandler {
	return &logHandler{
		inner: inner,
		reg:   reg,
		lgr:   global.GetLoggerProvider().Logger("github.com/cego/gitte"),
	}
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
	// Correlate to the task span if we know it.
	emitCtx := ctx
	if sc, ok := h.reg.Get(out.CmdName); ok && sc.IsValid() {
		emitCtx = trace.ContextWithSpanContext(ctx, sc)
	}
	h.lgr.Emit(emitCtx, rec)
}
