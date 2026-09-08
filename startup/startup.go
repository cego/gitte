package startup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"
	"github.com/cego/gitte/output"
	"github.com/cego/gitte/telemetry"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Run executes all startup checks and streams status to stdout.
// mode controls whether to use the plain text or TUI output.
func Run(ctx context.Context, cfg *config.GitteConfig, cwd string, mode output.OutputMode, names ...string) error {
	checks, err := selectChecks(cfg.StartupChecks, names)
	if err != nil {
		return err
	}
	if len(checks) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tasks := make([]executor.Task, 0, len(checks))
	for name, check := range checks {
		name := name
		check := check
		tasks = append(tasks, executor.Task{
			Name:  name,
			Needs: check.GetNeeds(),
			ExecuteFn: func(ctx context.Context, taskName string, handler executor.OutputHandler) (err error) {
				ctx, span := startCheckSpan(ctx, taskName)
				defer func() {
					if recovered := recover(); recovered != nil {
						panicErr := fmt.Errorf("panic: %v", recovered)
						span.RecordError(panicErr)
						span.SetStatus(codes.Error, panicErr.Error())
						span.End()
						panic(recovered)
					}
					if err != nil {
						span.RecordError(err)
						span.SetStatus(codes.Error, err.Error())
					}
					span.End()
				}()
				logHandler := telemetry.LogOutputHandler(handler)
				stdout := &handlerWriter{ctx: ctx, handler: logHandler, taskName: taskName, stream: executor.StdoutStream}
				stderr := &handlerWriter{ctx: ctx, handler: logHandler, taskName: taskName, stream: executor.StderrStream}
				if cerr := check.Check(ctx, cwd, stdout, stderr); cerr != nil {
					return describeFailure(ctx, check, name, cwd, cerr)
				}
				return nil
			},
		})
	}

	// Validate before starting a terminal program that needs completion events.
	if err := executor.ValidateNoCycles(tasks); err != nil {
		return fmt.Errorf("startup checks have invalid dependencies: %w", err)
	}
	// Build the view before creating the executor so we can pass hook closures.
	view := newView(mode, tasks, cancel)

	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{
		OnTaskStart:  view.OnStart,
		OnTaskFinish: view.OnFinish,
	})
	if err != nil {
		return fmt.Errorf("startup checks have invalid dependencies: %w", err)
	}
	runErr := exec.Execute(ctx)
	view.Wait()
	if runErr != nil {
		return errors.New("startup checks failed")
	}
	return runErr
}

// handlerWriter adapts startup checks using io.Writer to executor output while
// retaining the check span in the context used for correlated OTEL logs.
type handlerWriter struct {
	ctx      context.Context
	handler  executor.OutputHandler
	taskName string
	stream   executor.StreamType
}

var _ io.Writer = (*handlerWriter)(nil)

func (w *handlerWriter) Write(p []byte) (int, error) {
	line := append([]byte(nil), p...)
	_ = w.handler.HandleOutput(w.ctx, executor.Output{
		Output:  line,
		CmdName: w.taskName,
		Stream:  w.stream,
	})
	return len(p), nil
}

// startCheckSpan opens a span for a single startup check.
func startCheckSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return telemetry.Tracer().Start(ctx, "startup.check "+name)
}

// newView picks the right view implementation based on output mode.
func newView(mode output.OutputMode, tasks []executor.Task, cancel context.CancelFunc) View {
	if mode == output.ModePlain {
		return newPlainView(tasks)
	}

	// Collect names in a stable order for the TUI list.
	names := make([]string, 0, len(tasks))
	for _, t := range tasks {
		names = append(names, t.Name)
	}
	sort.Strings(names)

	return newTUIView(names, tasks, cancel)
}
