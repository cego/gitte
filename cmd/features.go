package cmd

import (
	"fmt"
	"strings"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/features"
	"github.com/cego/gitte/output"
	"github.com/cego/gitte/state"

	"github.com/spf13/cobra"
)

func newFeaturesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "features",
		Short: "Manage feature gates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputMode() == output.ModePlain {
				return newFeaturesListCmd().RunE(cmd, args)
			}
			return features.Run(globalCfg, globalCwd, globalSt)
		},
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
				scopeDesc := buildScopeDescription(gate.Scope.Projects, gate.Scope.GitlabGroups, gate.Scope.GithubOrgs)
				if fs, ok := globalSt.Features[gateName]; ok && fs.Enabled {
					enabled = "yes"
					if fs.OverrideScope != nil {
						scopeDesc = buildOverrideScopeDescription(fs.OverrideScope)
					}
				}

				fmt.Printf("%-30s %-8s %s\n", gateName, enabled, scopeDesc)
			}
			return nil
		},
	}
}

func newFeaturesEnableCmd() *cobra.Command {
	var (
		projects     []string
		gitlabGroups []string
		githubOrgs   []string
		excludes     []string
	)

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

			if len(projects) > 0 || len(gitlabGroups) > 0 || len(githubOrgs) > 0 {
				override := &state.ScopeOverride{Projects: projects}
				for _, g := range gitlabGroups {
					host, group, ok := strings.Cut(g, "/")
					if !ok {
						return fmt.Errorf("invalid --gitlab-group format %q, expected host/group", g)
					}
					entry := state.ScopeOverrideGroup{Host: host, Group: group, ExcludeProjects: excludes}
					override.GitlabGroups = append(override.GitlabGroups, entry)
				}
				for _, o := range githubOrgs {
					host, org, ok := strings.Cut(o, "/")
					if !ok {
						return fmt.Errorf("invalid --github-org format %q, expected host/org", o)
					}
					entry := state.ScopeOverrideOrg{Host: host, Org: org, ExcludeProjects: excludes}
					override.GithubOrgs = append(override.GithubOrgs, entry)
				}
				fs.OverrideScope = override
			}

			globalSt.Features[gateName] = fs
			if err := state.Save(globalCwd, globalSt); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}

			fmt.Printf("Feature gate %q enabled\n", gateName)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&projects, "project", nil, "limit to specific project(s)")
	cmd.Flags().StringArrayVar(&gitlabGroups, "gitlab-group", nil, "limit to gitlab group (host/group)")
	cmd.Flags().StringArrayVar(&githubOrgs, "github-org", nil, "limit to github org (host/org)")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "exclude project from all groups/orgs")
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

func buildOverrideScopeDescription(o *state.ScopeOverride) string {
	var parts []string
	if len(o.Projects) > 0 {
		parts = append(parts, "projects: "+strings.Join(o.Projects, ", "))
	}
	for _, g := range o.GitlabGroups {
		s := fmt.Sprintf("gitlab:%s/%s", g.Host, g.Group)
		if len(g.ExcludeProjects) > 0 {
			s += " (excl: " + strings.Join(g.ExcludeProjects, ", ") + ")"
		}
		parts = append(parts, s)
	}
	for _, g := range o.GithubOrgs {
		s := fmt.Sprintf("github:%s/%s", g.Host, g.Org)
		if len(g.ExcludeProjects) > 0 {
			s += " (excl: " + strings.Join(g.ExcludeProjects, ", ") + ")"
		}
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return "none (disabled)"
	}
	return strings.Join(parts, "; ")
}

func buildScopeDescription(projects []string, gitlabGroups []config.GitlabScope, githubOrgs []config.GithubScope) string {
	var parts []string
	if len(projects) > 0 {
		parts = append(parts, "projects: "+strings.Join(projects, ", "))
	}
	for _, g := range gitlabGroups {
		parts = append(parts, fmt.Sprintf("gitlab:%s/%s", g.Host, g.Group))
	}
	for _, g := range githubOrgs {
		parts = append(parts, fmt.Sprintf("github:%s/%s", g.Host, g.Org))
	}
	if len(parts) == 0 {
		return "all projects"
	}
	return strings.Join(parts, "; ")
}
