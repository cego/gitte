package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
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
	tr.OnFinish("a:build:sn")
	tr.OnFinish("b:build:sn") // last build task -> build span ends
	tr.OnStart("a:up:sn")
	tr.OnFinish("a:up:sn") // up span ends

	names := map[string]int{}
	for _, s := range exp.GetSpans() {
		names[s.Name]++
	}
	if names["build"] != 1 || names["up"] != 1 {
		t.Fatalf("want one build and one up span, got %v", names)
	}
}
