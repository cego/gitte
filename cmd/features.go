package cmd

import (
	"fmt"

	"gitte/state"

	"github.com/spf13/cobra"
)

func newFeaturesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "features",
		Short: "Manage feature gates",
	}

	cmd.AddCommand(
		newFeaturesListCmd(),
		newFeaturesEnableCmd(),
		newFeaturesDisableCmd(),
	)

	return cmd
}

func newFeaturesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all feature gates and their state",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("%-30s %-8s SCOPE\n", "GATE", "ENABLED")
			fmt.Printf("%-30s %-8s -----\n", "----", "-------")

			for gateName, gate := range globalCfg.FeatureGates {
				enabled := "no"
				if fs, ok := globalSt.Features[gateName]; ok && fs.Enabled {
					enabled = "yes"
				}

				// Build scope description
				scope := buildScopeDescription(gate.Scope.Projects, gate.Scope.GitlabGroups, gate.Scope.GithubOrgs)
				fmt.Printf("%-30s %-8s %s\n", gateName, enabled, scope)
			}
			return nil
		},
	}
}

func newFeaturesEnableCmd() *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "enable <gate>",
		Short: "Enable a feature gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gateName := args[0]

			if _, ok := globalCfg.FeatureGates[gateName]; !ok {
				return fmt.Errorf("unknown feature gate: %q", gateName)
			}

			fs := state.FeatureState{Enabled: true}
			if project != "" {
				fs.OverrideScope = &state.ScopeOverride{Projects: []string{project}}
			}

			globalSt.Features[gateName] = fs
			if err := state.Save(globalCwd, globalSt); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}

			fmt.Printf("Feature gate %q enabled\n", gateName)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "limit feature gate to a specific project")
	return cmd
}

func newFeaturesDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <gate>",
		Short: "Disable a feature gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gateName := args[0]

			if _, ok := globalSt.Features[gateName]; !ok {
				fmt.Printf("Feature gate %q was not enabled\n", gateName)
				return nil
			}

			delete(globalSt.Features, gateName)
			if err := state.Save(globalCwd, globalSt); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}

			fmt.Printf("Feature gate %q disabled\n", gateName)
			return nil
		},
	}
}

func buildScopeDescription(projects []string, gitlabGroups interface{}, githubOrgs interface{}) string {
	if len(projects) > 0 {
		result := "projects: "
		for i, p := range projects {
			if i > 0 {
				result += ", "
			}
			result += p
		}
		return result
	}
	return "all projects"
}
