package telemetry

import (
	"context"
	"testing"

	"github.com/cego/gitte/executor"
	"go.opentelemetry.io/otel/trace"
)

func TestSpanRegistry_SetGetDelete(t *testing.T) {
	reg := NewSpanRegistry()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{2},
	})
	reg.Set("proj:build:sn", sc)
	got, ok := reg.Get("proj:build:sn")
	if !ok || got.TraceID() != sc.TraceID() {
		t.Fatalf("Get = %v, %v; want %v", got, ok, sc)
	}
	reg.Delete("proj:build:sn")
	if _, ok := reg.Get("proj:build:sn"); ok {
		t.Fatal("expected entry removed after Delete")
	}
}

type recordingHandler struct{ lines []string }

func (r *recordingHandler) HandleOutput(_ context.Context, o executor.Output) error {
	r.lines = append(r.lines, string(o.Output))
	return nil
}

func TestLogOutputHandler_ForwardsUnchanged(t *testing.T) {
	// With logs disabled (no provider), the wrapper must still forward output
	// to the inner handler and never error.
	inner := &recordingHandler{}
	reg := NewSpanRegistry()
	h := LogOutputHandler(inner, reg)
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
