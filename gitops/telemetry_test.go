package gitops

import (
	"context"
	"strings"
	"testing"

	"github.com/cego/gitte/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestSetGitContextAttrs(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

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

func TestWithSyncSpan_DiscoveryUsesSafeRepoIdentity(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	remote := "https://user:secret@example.com/org/repo.git"
	if err := withSyncSpan(context.Background(), "example.com/org/repo", func(context.Context, trace.Span) error {
		return nil
	}); err != nil {
		t.Fatalf("withSyncSpan() error = %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "gitops.sync example.com/org/repo" {
		t.Fatalf("spans = %+v", spans)
	}
	var repo string
	for _, attr := range spans[0].Attributes {
		if attr.Key == "gitte.repo" {
			repo = attr.Value.AsString()
		}
	}
	if repo != "example.com/org/repo" {
		t.Fatalf("gitte.repo = %q, want sanitized repo identity", repo)
	}
	if strings.Contains(spans[0].Name, remote) {
		t.Fatalf("span name contains full remote URL: %q", spans[0].Name)
	}
	for _, attr := range spans[0].Attributes {
		if strings.Contains(attr.Value.AsString(), "secret") {
			t.Fatalf("span attribute contains remote credential: %v", attr)
		}
	}
}

func TestSyncProject_PanicRecordsErrorBeforePropagating(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		_ = syncProject(
			context.Background(), t.TempDir(), "example.com/org/repo",
			config.ProjectConfig{Remote: "https://example.com/org/repo.git"}, true,
			func(string) { panic("detail failed") }, func(CheckoutPrompt) {}, func(string) {},
		)
	}()
	if !panicked {
		t.Fatal("syncProject panic was swallowed")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("spans = %+v, want one error span", spans)
	}
}
