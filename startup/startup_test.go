package startup

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestStartCheckSpan_RecordsNamedSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	ctx, span := startCheckSpan(context.Background(), "git-present")
	span.End()
	_ = ctx

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "startup.check git-present" {
		t.Fatalf("got %+v, want one span named 'startup.check git-present'", spans)
	}
}
