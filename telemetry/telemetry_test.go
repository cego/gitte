package telemetry

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cego/gitte/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func TestResourceAttributes(t *testing.T) {
	find := func(attrs []attribute.KeyValue, key string) (string, bool) {
		for _, a := range attrs {
			if string(a.Key) == key {
				return a.Value.AsString(), true
			}
		}
		return "", false
	}

	t.Run("includes username and hostname when resolved", func(t *testing.T) {
		attrs := resourceAttributes("1.2.3", "alice", "dev-box")
		if v, _ := find(attrs, "service.name"); v != "gitte" {
			t.Errorf("service.name = %q, want gitte", v)
		}
		if v, _ := find(attrs, "service.version"); v != "1.2.3" {
			t.Errorf("service.version = %q, want 1.2.3", v)
		}
		if v, ok := find(attrs, "user.name"); !ok || v != "alice" {
			t.Errorf("user.name = %q (present=%v), want alice", v, ok)
		}
		if v, ok := find(attrs, "host.name"); !ok || v != "dev-box" {
			t.Errorf("host.name = %q (present=%v), want dev-box", v, ok)
		}
	})

	t.Run("omits username and hostname when empty", func(t *testing.T) {
		attrs := resourceAttributes("1.2.3", "", "")
		if _, ok := find(attrs, "user.name"); ok {
			t.Error("user.name should be omitted when empty")
		}
		if _, ok := find(attrs, "host.name"); ok {
			t.Error("host.name should be omitted when empty")
		}
	})
}

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
		if !r.Enabled {
			t.Fatalf("expected Enabled=true when GITTE_TELEMETRY_URL is set, got %+v", r)
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
	shutdown := Init(context.Background(), &config.GitteConfig{}, "test")
	if shutdown == nil {
		t.Fatal("shutdown must never be nil")
	}
	shutdown(context.Background()) // must not panic
}

func TestInit_EnabledReturnsCallableShutdown(t *testing.T) {
	// Verify that Init with a valid endpoint returns a non-nil shutdown function
	// that can be called without panicking or hanging (bounded flush timeout).
	// Note: otlptracehttp.New is lazy — it accepts any URL including unreachable
	// endpoints without error, so Init succeeds and returns a real shutdown.
	// The exporter-error branch (where New returns an error and Init falls back to
	// no-op) cannot be triggered deterministically with the HTTP exporter; the SDK
	// silently swallows malformed URLs and connection errors at export time.
	t.Setenv("GITTE_TELEMETRY", "")
	prev := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	cfg := &config.GitteConfig{Telemetry: config.TelemetryConfig{Endpoint: "http://localhost:4318"}}
	shutdown := Init(context.Background(), cfg, "test")
	if shutdown == nil {
		t.Fatal("shutdown must never be nil on the enabled path")
	}
	shutdown(context.Background()) // must not panic or hang beyond the flush timeout
}

func TestInit_RespectsOTELTracesSampler(t *testing.T) {
	t.Setenv("GITTE_TELEMETRY", "")
	t.Setenv("GITTE_TELEMETRY_LOGS", "off")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	prev := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	shutdown := Init(context.Background(), &config.GitteConfig{Telemetry: config.TelemetryConfig{Endpoint: "http://localhost:4318"}}, "test")
	_, span := Tracer().Start(context.Background(), "not-recorded")
	if span.IsRecording() {
		t.Fatal("span is recording despite OTEL_TRACES_SAMPLER=always_off")
	}
	span.End()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	shutdown(ctx)
}

type testShutdowner struct {
	called chan struct{}
	wait   bool
	once   sync.Once
}

func (s *testShutdowner) Shutdown(ctx context.Context) error {
	s.once.Do(func() { close(s.called) })
	if s.wait {
		<-ctx.Done()
	}
	return nil
}

func TestShutdownProviders_RunIndependentlyAndHonorCancellation(t *testing.T) {
	traceProvider := &testShutdowner{called: make(chan struct{}), wait: true}
	logProvider := &testShutdowner{called: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		shutdownProviders(ctx, traceProvider, logProvider)
		close(done)
	}()

	select {
	case <-logProvider.called:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("log shutdown was blocked behind trace shutdown")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shutdown did not stop after cancellation")
	}
}

func TestStartCommandSpan_NoProviderDoesNotPanic(t *testing.T) {
	// With no provider set, Tracer() returns a no-op tracer; span ops are safe.
	_, span := StartCommandSpan(context.Background(), "gitte run", []string{"up"})
	span.End()
}

func TestStartPhaseSpan_ReturnsChildContext(t *testing.T) {
	ctx, span := StartPhaseSpan(context.Background(), "startup")
	if span == nil {
		t.Fatal("nil span")
	}
	if ctx == context.Background() {
		t.Fatal("expected a derived context")
	}
	span.End()
}
