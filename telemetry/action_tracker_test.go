package telemetry

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestActionTracker_SpanPerAction(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())
	tr.OnStart("a:build:sn")
	tr.OnStart("b:build:sn")
	tr.OnFinish("a:build:sn", nil)
	tr.OnFinish("b:build:sn", nil) // last build task -> build span ends
	tr.OnStart("a:up:sn")
	tr.OnFinish("a:up:sn", nil) // up span ends

	names := map[string]int{}
	for _, s := range exp.GetSpans() {
		names[s.Name]++
	}
	if names["build"] != 1 || names["up"] != 1 {
		t.Fatalf("want one build and one up span, got %v", names)
	}
}

func TestActionTracker_SkippedTaskDoesNotCloseSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())

	// Two real tasks start.
	tr.OnStart("a:build:sn")
	tr.OnStart("b:build:sn")

	// A skipped task (never started) fires OnFinish — must be a no-op.
	tr.OnFinish("c:build:sn", nil)

	// Action span must still be open: ActionContext returns the action's own
	// context (not the phase/background context), and no "build" span exported.
	ctxAfterSkip := tr.ActionContext("build")
	if ctxAfterSkip == context.Background() {
		t.Fatal("action span was prematurely closed by skipped task's OnFinish")
	}
	if n := countSpans(exp, "build"); n != 0 {
		t.Fatalf("want 0 exported build spans after skipped OnFinish, got %d", n)
	}

	// One real task finishes — span still open because b:build:sn is active.
	tr.OnFinish("a:build:sn", nil)
	if n := countSpans(exp, "build"); n != 0 {
		t.Fatalf("want 0 exported build spans after first real OnFinish, got %d", n)
	}

	// Last real task finishes — span must close now, exactly once.
	tr.OnFinish("b:build:sn", nil)
	if n := countSpans(exp, "build"); n != 1 {
		t.Fatalf("want exactly 1 exported build span after last real OnFinish, got %d", n)
	}

	// ActionContext must now fall back to the phase context.
	if tr.ActionContext("build") != context.Background() {
		t.Fatal("expected action context to fall back to phase context after span closed")
	}
}

func countSpans(exp *tracetest.InMemoryExporter, name string) int {
	n := 0
	for _, s := range exp.GetSpans() {
		if s.Name == name {
			n++
		}
	}
	return n
}

func TestActionTracker_RecordsTaskErrorOnActionSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())
	tr.OnStart("a:build:sn")
	tr.OnStart("b:build:sn")
	tr.OnFinish("a:build:sn", errors.New("build failed")) // one task fails
	tr.OnFinish("b:build:sn", nil)                          // last finishes -> span ends

	spans := exp.GetSpans()
	var build *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "build" {
			build = &spans[i]
		}
	}
	if build == nil {
		t.Fatal("no build action span exported")
	}
	if build.Status.Code != codes.Error {
		t.Fatalf("build span status = %v, want Error", build.Status.Code)
	}
}
