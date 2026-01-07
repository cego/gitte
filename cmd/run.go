package cmd

import (
	"fmt"
	"gitte/config"
	"gitte/internal"

	"github.com/spf13/cobra"
)

// actionsCmd represents the actions command
var runCmd = &cobra.Command{
	Use: "run <action> <group> [project]",
	Annotations: map[string]string{
		"need-config": "true",
	},
	Short: "Execute actions on projects in groups after running startup checks and gitops",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := config.ConfigFromContext(ctx)

		// Parse and validate action argument
		if args[0] == "" {
			return fmt.Errorf("action cannot be empty")
		}
		actionString := args[0]

		// Parse and validate group argument
		if args[1] == "" {
			return fmt.Errorf("group cannot be empty")
		}
		groupString := args[1]

		// Parse and validate project argument. If project is not provided, use '*' to indicate all projects.
		projectString := "*"
		if len(args) > 2 {
			if args[2] == "" {
				return fmt.Errorf("project cannot be empty")
			}
			projectString = args[2]
		}

		fmt.Printf("Executing action [%s] for group [%s] on project [%s]\n", actionString, groupString, projectString)
		executor := internal.CreateActionExecutors(*cfg, true, actionString, groupString, projectString)

		// Check that executors are valid before running startup checks and gitops, to fail fast.
		// They are valid if there is at least one executor to run, and all executors have >0 tasks to execute.
		if len(executor) == 0 {
			return fmt.Errorf("no executors found for action '%s', group '%s', project '%s'", actionString, groupString, projectString)
		}
		for _, actionExecutor := range executor {
			if len(actionExecutor.Executor.GetPendingCommands()) == 0 {
				return fmt.Errorf("no tasks to execute for action '%s' on group '%s', project '%s'", actionExecutor.Action, groupString, projectString)
			}
		}

		// Run startup checks and gitops before executing actions
		if err := internal.PerformStartupChecks(ctx, config.CwdFromContext(ctx), *cfg); err != nil {
			return fmt.Errorf("startup checks failed: %w", err)
		}
		fmt.Println("Startup checks completed successfully.")
		if err := internal.GitOps(ctx, config.CwdFromContext(ctx), *cfg); err != nil {
			return fmt.Errorf("gitops failed: %w", err)
		}
		fmt.Println("GitOps completed successfully.")

		for _, actionExecutor := range executor {
			fmt.Printf("Running executor for action: %s\n", actionExecutor.Action)
			if err := actionExecutor.Executor.Execute(ctx); err != nil {
				return fmt.Errorf("failed to execute action '%s': %w", actionExecutor.Action, err)
			}
			fmt.Printf("Successfully executed action: %s\n", actionExecutor.Action)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
