package internal

import (
	"context"
	"gitte/config"
	"gitte/executor"
)

func PerformStartupChecks(ctx context.Context, cwd string, gitteConfig config.GitteConfig) error {
	tasks := []executor.Task{}
	for name, check := range gitteConfig.StartupChecks {
		tasks = append(tasks, executor.Task{
			Name: name,
			ExecuteFn: func(ctx context.Context, name string, oh executor.OutputHandler) error {
				return check.Check(ctx, cwd)
			},
			Needs: check.GetNeeds(),
		})
	}

	return executor.NewExecutor(tasks).Execute(ctx)
}
