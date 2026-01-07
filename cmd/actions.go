package cmd

import (
	"fmt"
	"gitte/config"
	"gitte/internal"

	"github.com/spf13/cobra"
)

// actionsCmd represents the actions command
var actionsCmd = &cobra.Command{
	Use: "actions <action> <group> [project]",
	Annotations: map[string]string{
		"need-config": "true",
	},
	Short: "Execute actions on projects in groups.",
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
	rootCmd.AddCommand(actionsCmd)
}
