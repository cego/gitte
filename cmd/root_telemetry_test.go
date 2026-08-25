package cmd

import (
	"context"
	"testing"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/telemetry"
	"go.opentelemetry.io/otel"
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

func TestRootCmd_EarlyConfigFailureUsesEnvironmentTelemetry(t *testing.T) {
	t.Setenv("GITTE_TELEMETRY", "")
	t.Setenv("GITTE_TELEMETRY_URL", "http://telemetry.example:4318")

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		_ = provider.Shutdown(context.Background())
	})

	previousInit := initTelemetry
	previousCtx := globalCtx
	previousSpan := globalRootSpan
	previousShutdown := globalTelemetryShutdown
	previousCancel := globalCancel
	previousCwd := flagCwd
	previousConfigPath := flagConfigPath
	var createdCancel context.CancelFunc
	t.Cleanup(func() {
		if createdCancel != nil {
			createdCancel()
		}
		initTelemetry = previousInit
		globalCtx = previousCtx
		globalRootSpan = previousSpan
		globalTelemetryShutdown = previousShutdown
		globalCancel = previousCancel
		flagCwd = previousCwd
		flagConfigPath = previousConfigPath
	})

	initCalled := false
	initTelemetry = func(ctx context.Context, cfg *config.GitteConfig, version string) func(context.Context) {
		initCalled = true
		if cfg != nil {
			t.Fatalf("early config telemetry must not use partial config: %+v", cfg)
		}
		if resolved := telemetry.Resolve(cfg); !resolved.Enabled || resolved.Endpoint != "http://telemetry.example:4318" {
			t.Fatalf("environment telemetry was not resolved: %+v", resolved)
		}
		return func(context.Context) {}
	}

	flagCwd = t.TempDir()
	flagConfigPath = ""
	err := rootCmd.PersistentPreRunE(rootCmd, nil)
	createdCancel = globalCancel
	if err == nil {
		t.Fatal("PersistentPreRunE succeeded without a config")
	}
	if !initCalled || globalRootSpan == nil || !globalRootSpan.IsRecording() {
		t.Fatal("root telemetry span was not started after config failure")
	}
	finishTelemetry(err)
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("spans = %+v, want one error root span", spans)
	}
}
