package telemetry

import (
	"context"
	"os"
	"testing"

	"github.com/cego/gitte/config"
)

func TestResolve_Precedence(t *testing.T) {
	// Save and clear env that influences resolution.
	for _, k := range []string{"GITTE_TELEMETRY", "GITTE_TELEMETRY_URL", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"} {
		t.Setenv(k, "")
	}

	cfgWith := func(ep string) *config.GitteConfig {
		return &config.GitteConfig{Telemetry: config.TelemetryConfig{Endpoint: ep, Headers: map[string]string{"Authorization": "Bearer x"}}}
	}

	t.Run("disabled when no endpoint anywhere", func(t *testing.T) {
		r := Resolve(&config.GitteConfig{})
		if r.Enabled {
			t.Fatal("expected disabled")
		}
	})

	t.Run("enabled from config endpoint", func(t *testing.T) {
		r := Resolve(cfgWith("https://apm:8200"))
		if !r.Enabled || r.Endpoint != "https://apm:8200" || r.UseSDKEnv {
			t.Fatalf("got %+v", r)
		}
		if r.Headers["Authorization"] != "Bearer x" {
			t.Fatalf("headers not carried: %+v", r.Headers)
		}
	})

	t.Run("GITTE_TELEMETRY_URL overrides config", func(t *testing.T) {
		t.Setenv("GITTE_TELEMETRY_URL", "https://override:8200")
		r := Resolve(cfgWith("https://apm:8200"))
		if r.Endpoint != "https://override:8200" {
			t.Fatalf("got %+v", r)
		}
	})

	t.Run("GITTE_TELEMETRY=off disables everything", func(t *testing.T) {
		t.Setenv("GITTE_TELEMETRY", "off")
		t.Setenv("GITTE_TELEMETRY_URL", "https://override:8200")
		r := Resolve(cfgWith("https://apm:8200"))
		if r.Enabled {
			t.Fatalf("expected disabled, got %+v", r)
		}
	})

	t.Run("falls back to OTEL env endpoint with UseSDKEnv", func(t *testing.T) {
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://otel:4318")
		r := Resolve(&config.GitteConfig{})
		if !r.Enabled || !r.UseSDKEnv || r.Endpoint != "" {
			t.Fatalf("got %+v", r)
		}
	})
}

func TestInit_DisabledReturnsNoopShutdown(t *testing.T) {
	t.Setenv("GITTE_TELEMETRY", "off")
	shutdown, err := Init(context.Background(), &config.GitteConfig{}, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must never be nil")
	}
	shutdown() // must not panic
}

func TestStartCommandSpan_NoProviderDoesNotPanic(t *testing.T) {
	// With no provider set, Tracer() returns a no-op tracer; span ops are safe.
	_, span := StartCommandSpan(context.Background(), "gitte run", []string{"up"})
	span.End()
	_ = os.Getenv // keep import
}
