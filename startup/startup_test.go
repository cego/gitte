package startup

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
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

type startupLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *startupLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (*startupLogExporter) Shutdown(context.Context) error   { return nil }
func (*startupLogExporter) ForceFlush(context.Context) error { return nil }

func TestRun_ExportsStartupCommandOutputWithSpanCorrelation(t *testing.T) {
	logExp := &startupLogExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logExp)))
	prevLP := global.GetLoggerProvider()
	global.SetLoggerProvider(lp)
	t.Cleanup(func() { global.SetLoggerProvider(prevLP); _ = lp.Shutdown(context.Background()) })

	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prevTP); _ = tp.Shutdown(context.Background()) })

	cfg := &config.GitteConfig{StartupChecks: config.StartupCheckMap{
		"output-check": &config.ShellStartupCheck{
			BaseStartupCheck: config.BaseStartupCheck{Type: "shell"},
			Shell:            "sh",
			Script:           "printf stdout-line; printf stderr-line >&2",
		},
	}}
	if err := Run(context.Background(), cfg, t.TempDir(), output.ModePlain); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(logExp.records) != 2 {
		t.Fatalf("exported %d startup log records, want 2", len(logExp.records))
	}
	for _, record := range logExp.records {
		if !record.TraceID().IsValid() || !record.SpanID().IsValid() {
			t.Fatalf("startup log is not span-correlated: trace=%s span=%s", record.TraceID(), record.SpanID())
		}
	}
}

func TestRun_RecordsFailedNamesWithoutGuidance(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	cfg, err := config.LoadGitteConfigFromYAML([]byte(`startup:
  z-failure:
    type: shell
    shell: sh
    script: "printf failure-diagnostic >&2; exit 1"
    guidance:
      shell: sh
      script: "printf private-guidance-marker"
  a-failure:
    type: shell
    shell: sh
    script: "exit 1"
    hint: private-hint-marker
  passed:
    type: shell
    shell: sh
    script: "true"
  blocked:
    type: shell
    shell: sh
    script: "true"
    needs: [z-failure]
`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, parent := tp.Tracer("test").Start(context.Background(), "startup")
	err = Run(ctx, cfg, t.TempDir(), output.ModePlain)
	parent.End()
	if err == nil || err.Error() != "startup checks failed" {
		t.Fatalf("Run() error = %v", err)
	}

	foundNames, foundCause := false, false
	for _, span := range exp.GetSpans() {
		if span.Name == "startup" {
			for _, attr := range span.Attributes {
				if attr.Key == "startup.failed_checks" {
					got := attr.Value.AsStringSlice()
					if !slices.Equal(got, []string{"a-failure", "z-failure"}) {
						t.Errorf("failed checks = %v", got)
					}
					foundNames = true
				}
			}
		}
		if span.Name == "startup.check z-failure" && strings.Contains(span.Status.Description, "failure-diagnostic") {
			foundCause = true
		}
		if text := fmt.Sprintf("%+v", span); strings.Contains(text, "private-guidance-marker") || strings.Contains(text, "private-hint-marker") {
			t.Error("guidance or hint leaked into telemetry")
		}
	}
	if !foundNames || !foundCause {
		t.Fatalf("missing failure telemetry: parent names=%t, child cause=%t", foundNames, foundCause)
	}
}
