package internal

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
)

func PerformStartupChecks(ctx context.Context, cwd string, gitteConfig config.GitteConfig) error {
	tasks := []executor.Task{}
	for name, check := range gitteConfig.StartupChecks {
		tasks = append(tasks, executor.Task{
			Name: fmt.Sprintf("startup-check-%s", name),
			ExecuteFn: func() error {
				return check.Check(ctx, cwd)
			},
		})
	}

	return executor.NewExecutor(tasks).Execute()
}
