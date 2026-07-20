package cmd

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRunWithTelemetry_RecordsAndFlushesPanicThenRepanics(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	_, span := provider.Tracer("test").Start(context.Background(), "gitte test")
	previousSpan := globalRootSpan
	previousShutdown := globalTelemetryShutdown
	globalRootSpan = span
	shutdownCalled := false
	globalTelemetryShutdown = func(context.Context) { shutdownCalled = true }
	t.Cleanup(func() {
		globalRootSpan = previousSpan
		globalTelemetryShutdown = previousShutdown
	})

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_ = runWithTelemetry(func() error { panic("boom") })
	}()

	if recovered != "boom" {
		t.Fatalf("recovered panic = %v, want boom", recovered)
	}
	if !shutdownCalled {
		t.Fatal("telemetry shutdown was not called")
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("root span status = %v, want error", spans[0].Status.Code)
	}
	foundException := false
	for _, event := range spans[0].Events {
		if event.Name == "exception" {
			foundException = true
		}
	}
	if !foundException {
		t.Fatal("panic was not sent as an exception event")
	}
}
