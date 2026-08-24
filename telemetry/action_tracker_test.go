package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cego/gitte/executor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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
	tr.OnFinish("a:build:sn", nil)
	tr.OnFinish("b:build:sn", nil)
	tr.OnStart("a:up:sn")
	tr.OnFinish("a:up:sn", nil)
	tr.Close()

	names := map[string]int{}
	for _, s := range exp.GetSpans() {
		names[s.Name]++
	}
	if names["build"] != 1 || names["up"] != 1 {
		t.Fatalf("want one build and one up span, got %v", names)
	}
}

func TestActionTracker_SkippedTaskDoesNotCloseSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())

	// Two real tasks start.
	tr.OnStart("a:build:sn")
	tr.OnStart("b:build:sn")

	// A skipped task (never started) fires OnFinish — must be a no-op.
	tr.OnFinish("c:build:sn", nil)

	// Action span must still be open: ActionContext returns the action's own
	// context (not the phase/background context), and no "build" span exported.
	ctxAfterSkip := tr.ActionContext("build")
	if ctxAfterSkip == context.Background() {
		t.Fatal("action span was prematurely closed by skipped task's OnFinish")
	}
	if n := countSpans(exp, "build"); n != 0 {
		t.Fatalf("want 0 exported build spans after skipped OnFinish, got %d", n)
	}

	// One real task finishes — span still open because b:build:sn is active.
	tr.OnFinish("a:build:sn", nil)
	if n := countSpans(exp, "build"); n != 0 {
		t.Fatalf("want 0 exported build spans after first real OnFinish, got %d", n)
	}

	// Last real task finishes — the explicit close has not happened yet.
	tr.OnFinish("b:build:sn", nil)
	if n := countSpans(exp, "build"); n != 0 {
		t.Fatalf("want 0 exported build spans before Close, got %d", n)
	}

	tr.Close()
	if n := countSpans(exp, "build"); n != 1 {
		t.Fatalf("want exactly 1 exported build span after Close, got %d", n)
	}

	// ActionContext must now fall back to the phase context.
	if tr.ActionContext("build") != context.Background() {
		t.Fatal("expected action context to fall back to phase context after span closed")
	}
}

func countSpans(exp *tracetest.InMemoryExporter, name string) int {
	n := 0
	for _, s := range exp.GetSpans() {
		if s.Name == name {
			n++
		}
	}
	return n
}

func runExecutorWithTracker(t *testing.T, tasks []executor.Task) (*tracetest.InMemoryExporter, error) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tracker := NewActionTracker(context.Background())
	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{
		OnTaskStart: tracker.OnStart,
		OnTaskFinish: func(name string, err error, _ time.Duration) {
			tracker.OnFinish(name, err)
		},
	})
	if err != nil {
		return exp, err
	}
	runErr := exec.Execute(context.Background())
	tracker.Close()
	return exp, runErr
}

func TestActionTracker_ExecutorDependencyChainUsesOneActionSpan(t *testing.T) {
	exp, err := runExecutorWithTracker(t, []executor.Task{
		{Name: "a:build:base", ExecuteFn: func(context.Context, string, executor.OutputHandler) error {
			return nil
		}},
		{Name: "b:build:dependent", Needs: []string{"a:build:base"}, ExecuteFn: func(context.Context, string, executor.OutputHandler) error {
			return nil
		}},
	})
	if err != nil {
		t.Fatalf("executor returned error: %v", err)
	}
	if got := countSpans(exp, "build"); got != 1 {
		t.Fatalf("want exactly one build action span, got %d", got)
	}
}

func TestActionTracker_ExecutorRetrySuccessLeavesActionSpanSuccessful(t *testing.T) {
	var attempts int32
	exp, err := runExecutorWithTracker(t, []executor.Task{{
		Name:  "a:build:retry",
		Retry: executor.RetryConfig{Attempts: 2, Delay: "0s"},
		ExecuteFn: func(context.Context, string, executor.OutputHandler) error {
			if attempts++; attempts == 1 {
				return errors.New("transient")
			}
			return nil
		},
	}})
	if err != nil {
		t.Fatalf("executor returned error after retry: %v", err)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "build" {
		t.Fatalf("want exactly one build action span, got %+v", spans)
	}
	if spans[0].Status.Code == codes.Error {
		t.Fatalf("retry-success action span status = Error, want non-Error")
	}
}

func TestActionTracker_ExecutorTerminalFailureLeavesActionSpanError(t *testing.T) {
	exp, err := runExecutorWithTracker(t, []executor.Task{{
		Name: "a:build:broken",
		ExecuteFn: func(context.Context, string, executor.OutputHandler) error {
			return errors.New("terminal")
		},
	}})
	if err == nil {
		t.Fatal("expected executor error")
	}
	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].Name != "build" {
		t.Fatalf("want exactly one build action span, got %+v", spans)
	}
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("terminal-failure action span status = %v, want Error", spans[0].Status.Code)
	}
}

func TestActionTracker_RecordsTaskErrorOnActionSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())
	tr.OnStart("a:build:sn")
	tr.OnFinish("a:build:sn", errors.New("build failed")) // first attempt fails
	tr.OnStart("a:build:sn")
	tr.OnFinish("a:build:sn", nil) // later retry succeeds
	tr.OnStart("b:build:sn")
	tr.OnFinish("b:build:sn", errors.New("another build failed"))
	tr.Close()

	spans := exp.GetSpans()
	var build *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "build" {
			build = &spans[i]
		}
	}
	if build == nil {
		t.Fatal("no build action span exported")
	}
	if build.Status.Code != codes.Error {
		t.Fatalf("build span status = %v, want Error", build.Status.Code)
	}
	if len(build.Events) != 2 {
		t.Fatalf("want one error event per failed attempt, got %d", len(build.Events))
	}
}

func TestActionTracker_SequentialTasksUseOneSpanUntilClose(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())
	tr.OnStart("a:build:sn")
	tr.OnFinish("a:build:sn", nil)
	tr.OnStart("b:build:sn")
	tr.OnFinish("b:build:sn", nil)
	if n := countSpans(exp, "build"); n != 0 {
		t.Fatalf("want no build span until tracker close, got %d", n)
	}
	tr.Close()
	if n := countSpans(exp, "build"); n != 1 {
		t.Fatalf("want one build span for sequential tasks, got %d", n)
	}
}

func TestActionTracker_RetryUsesSameSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	tr := NewActionTracker(context.Background())
	tr.OnStart("a:build:sn")
	tr.OnFinish("a:build:sn", errors.New("transient"))
	tr.OnStart("a:build:sn")
	tr.OnFinish("a:build:sn", nil)
	tr.Close()

	if n := countSpans(exp, "build"); n != 1 {
		t.Fatalf("want one build span across retry, got %d", n)
	}
	spans := exp.GetSpans()
	if len(spans[0].Events) != 1 {
		t.Fatalf("want one error event for the failed attempt, got %d", len(spans[0].Events))
	}
}
