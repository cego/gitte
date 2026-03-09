package startup

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
	"gitte/output"
	"sort"
)

// Run executes all startup checks and streams status to stdout.
// mode controls whether to use the plain text or TUI output.
func Run(ctx context.Context, cfg *config.GitteConfig, cwd string, mode output.OutputMode) error {
	if len(cfg.StartupChecks) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tasks := make([]executor.Task, 0, len(cfg.StartupChecks))
	for name, check := range cfg.StartupChecks {
		name := name
		check := check
		tasks = append(tasks, executor.Task{
			Name:  name,
			Needs: check.GetNeeds(),
			ExecuteFn: func(ctx context.Context, taskName string, handler executor.OutputHandler) error {
				if err := check.Check(ctx, cwd); err != nil {
					hint := check.GetHint()
					if hint != "" {
						return fmt.Errorf("%s\nhint: %s", err.Error(), hint)
					}
					return err
				}
				return nil
			},
		})
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
	return runErr
}

// newView picks the right view implementation based on output mode.
func newView(mode output.OutputMode, tasks []executor.Task, cancel context.CancelFunc) View {
	if mode == output.ModePlain {
		return newPlainView()
	}

	// Collect names in a stable order for the TUI list.
	names := make([]string, 0, len(tasks))
	for _, t := range tasks {
		names = append(names, t.Name)
	}
	sort.Strings(names)

	return newTUIView(names, cancel)
}
