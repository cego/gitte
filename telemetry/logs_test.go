package telemetry

import (
	"context"
	"sync"
	"testing"

	"github.com/cego/gitte/executor"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type recordingHandler struct{ lines []string }

func (r *recordingHandler) HandleOutput(_ context.Context, o executor.Output) error {
	r.lines = append(r.lines, string(o.Output))
	return nil
}

func TestLogOutputHandler_ForwardsUnchanged(t *testing.T) {
	// With logs disabled (no provider), the wrapper must still forward output
	// to the inner handler and never error.
	inner := &recordingHandler{}
	h := LogOutputHandler(inner)
	err := h.HandleOutput(context.Background(), executor.Output{
		Output: []byte("hello"), CmdName: "proj:build:sn", Stream: executor.StdoutStream,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inner.lines) != 1 || inner.lines[0] != "hello" {
		t.Fatalf("inner did not receive line: %+v", inner.lines)
	}
}

// recordingExporter is a minimal sdklog.Exporter that captures exported records.
type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(_ context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(_ context.Context) error { return nil }

func (e *recordingExporter) Records() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]sdklog.Record, len(e.records))
	copy(out, e.records)
	return out
}

// setupRecordingProvider installs a real LoggerProvider backed by exp as the
// global, and returns a cleanup function that restores the previous global.
func setupRecordingProvider(t *testing.T) *recordingExporter {
	t.Helper()
	exp := &recordingExporter{}
	proc := sdklog.NewSimpleProcessor(exp)
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(proc))

	prev := global.GetLoggerProvider()
	global.SetLoggerProvider(provider)
	t.Cleanup(func() {
		global.SetLoggerProvider(prev)
	})
	return exp
}

func TestLogOutputHandler_StdoutSeverityInfo(t *testing.T) {
	exp := setupRecordingProvider(t)

	inner := &recordingHandler{}
	h := LogOutputHandler(inner)

	_ = h.HandleOutput(context.Background(), executor.Output{
		Output:  []byte("normal line"),
		CmdName: "task",
		Stream:  executor.StdoutStream,
	})

	recs := exp.Records()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if got := recs[0].Severity(); got != log.SeverityInfo {
		t.Errorf("stdout severity = %v; want SeverityInfo (%v)", got, log.SeverityInfo)
	}
}

func TestLogOutputHandler_StderrSeverityWarn(t *testing.T) {
	exp := setupRecordingProvider(t)

	inner := &recordingHandler{}
	h := LogOutputHandler(inner)

	_ = h.HandleOutput(context.Background(), executor.Output{
		Output:  []byte("error output"),
		CmdName: "task",
		Stream:  executor.StderrStream,
	})

	recs := exp.Records()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if got := recs[0].Severity(); got != log.SeverityWarn {
		t.Errorf("stderr severity = %v; want SeverityWarn (%v)", got, log.SeverityWarn)
	}
}

func TestLogOutputHandler_HintAttribute(t *testing.T) {
	exp := setupRecordingProvider(t)

	inner := &recordingHandler{}
	h := LogOutputHandler(inner)

	_ = h.HandleOutput(context.Background(), executor.Output{
		Output:  []byte("[HINT] do something"),
		CmdName: "task",
		Stream:  executor.StdoutStream,
	})

	recs := exp.Records()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}

	var hintVal log.Value
	var found bool
	recs[0].WalkAttributes(func(kv log.KeyValue) bool {
		if kv.Key == "gitte.hint" {
			hintVal = kv.Value
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatal("expected attribute gitte.hint=true but it was not present")
	}
	if !hintVal.AsBool() {
		t.Errorf("gitte.hint = %v; want true", hintVal)
	}
}

func TestLogOutputHandler_UsesProducerSpanContext(t *testing.T) {
	exp := setupRecordingProvider(t)
	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "task")
	wantTraceID := span.SpanContext().TraceID()
	wantSpanID := span.SpanContext().SpanID()
	h := LogOutputHandler(&recordingHandler{})
	_ = h.HandleOutput(ctx, executor.Output{Output: []byte("last line"), CmdName: "task", Stream: executor.StderrStream})
	span.End()

	recs := exp.Records()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].TraceID() != wantTraceID || recs[0].SpanID() != wantSpanID {
		t.Fatalf("log correlation = %s/%s, want %s/%s", recs[0].TraceID(), recs[0].SpanID(), wantTraceID, wantSpanID)
	}
}

func TestSignalEndpointURL(t *testing.T) {
	cases := []struct{ endpoint, signal, want string }{
		{"https://apm.example.com", "traces", "https://apm.example.com/v1/traces"},
		{"https://apm.example.com/", "traces", "https://apm.example.com/v1/traces"},
		{"https://apm.example.com:8200/", "logs", "https://apm.example.com:8200/v1/logs"},
		{"https://apm.example.com/custom/traces", "traces", "https://apm.example.com/custom/traces"},
		{"https://apm.example.com/?token=x", "traces", "https://apm.example.com/v1/traces?token=x"},
	}
	for _, c := range cases {
		if got := signalEndpointURL(c.endpoint, c.signal); got != c.want {
			t.Errorf("signalEndpointURL(%q, %q) = %q, want %q", c.endpoint, c.signal, got, c.want)
		}
	}
}
