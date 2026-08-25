package actions

import (
	"context"
	"testing"

	"github.com/cego/gitte/config"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestSetActionAttrs(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	_, span := tp.Tracer("test").Start(context.Background(), "action.run")
	setActionAttrs(span, "proj:up:default", "proj", "docker compose up")
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	got := map[string]string{}
	for _, kv := range spans[0].Attributes {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	if got["gitte.task"] != "proj:up:default" || got["gitte.project"] != "proj" || got["gitte.command"] != "docker compose up" {
		t.Fatalf("attrs = %+v", got)
	}
}

func TestSetTaskTelemetryAttrs_SkipsWorkForNonRecordingSpan(t *testing.T) {
	_, span := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "task")
	// nil config/state would panic in feature and environment resolution. A
	// non-recording span must return before touching either dependency.
	setTaskTelemetryAttrs(span, nil, nil, "project", config.ProjectConfig{}, "project:up:default", []string{"true"})
}
