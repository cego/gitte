package gitops

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetGitContextAttrs(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	_, span := tp.Tracer("test").Start(context.Background(), "gitops.sync")
	setGitContextAttrs(span, "main", "abc123", true)
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	attrs := map[string]string{}
	dirty := false
	for _, kv := range spans[0].Attributes {
		switch kv.Key {
		case "git.branch":
			attrs["branch"] = kv.Value.AsString()
		case "git.sha":
			attrs["sha"] = kv.Value.AsString()
		case "git.dirty":
			dirty = kv.Value.AsBool()
		}
	}
	if attrs["branch"] != "main" || attrs["sha"] != "abc123" || !dirty {
		t.Fatalf("attrs = %+v dirty=%v", attrs, dirty)
	}
}
