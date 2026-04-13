package cmd

import (
	"fmt"

	"github.com/cego/gitte/config"
	"github.com/spf13/cobra"
)

func newSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage local discovery sources",
		Long: `Manage the GitLab groups and GitHub orgs that 'gitte gitops --discover' queries.

Sources are stored in .gitte-override.yml alongside the main .gitte.yml so
they stay local to your machine and do not affect shared configuration.

Quick setup:
  gitte sources add gitlab gitlab.example.com mygroup subgroup
  gitte sources add github github.com myorg
  gitte gitops --discover

Running 'gitte sources' without a subcommand is equivalent to 'gitte sources list'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return newSourcesListCmd().RunE(cmd, args)
		},
	}
	cmd.AddCommand(
		newSourcesListCmd(),
		newSourcesAddCmd(),
		newSourcesRemoveCmd(),
	)
	return cmd
}

func newSourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List local discovery sources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			override, err := config.LoadOverrideConfig(globalCwd)
			if err != nil {
				return fmt.Errorf("failed to load local override: %w", err)
			}

			if len(override.Sources.Gitlab) == 0 && len(override.Sources.Github) == 0 {
				fmt.Println("No local sources configured. Use 'gitte sources add' to add sources.")
				return nil
			}

			for _, src := range override.Sources.Gitlab {
				fmt.Printf("gitlab  %s  [%s]\n", src.Host, tokenLabel(src.TokenEnv, src.TokenCmd))
				for _, g := range src.Groups {
					fmt.Printf("  %s\n", g)
				}
			}
			for _, src := range override.Sources.Github {
				fmt.Printf("github  %s  [%s]\n", src.Host, tokenLabel(src.TokenEnv, src.TokenCmd))
				for _, org := range src.Orgs {
					fmt.Printf("  %s\n", org)
				}
			}
			return nil
		},
	}
}

func newSourcesAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add groups or orgs to local discovery sources",
	}
	cmd.AddCommand(
		newSourcesAddGitlabCmd(),
		newSourcesAddGithubCmd(),
	)
	return cmd
}

func newSourcesAddGitlabCmd() *cobra.Command {
	var tokenEnv, tokenCmd string
	cmd := &cobra.Command{
		Use:   "gitlab <host> <group> [group...]",
		Short: "Add GitLab groups to local discovery sources",
		Long: `Add one or more GitLab groups to local discovery sources.

By default the token is looked up from the system keyring. Run:
  gitte token set gitlab <host>

to store a token with read_api scope. Without a token, only public groups are accessible.

Examples:
  gitte sources add gitlab gitlab.example.com mygroup
  gitte sources add gitlab gitlab.example.com groupA groupB
  gitte sources add gitlab gitlab.example.com mygroup --token-env MY_TOKEN
  gitte sources add gitlab gitlab.example.com mygroup --token-cmd "pass show gitlab/token"`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			groups := args[1:]

			override, err := config.LoadOverrideConfig(globalCwd)
			if err != nil {
				return fmt.Errorf("failed to load local override: %w", err)
			}

			added := 0
			found := false
			for i, src := range override.Sources.Gitlab {
				if src.Host != host {
					continue
				}
				found = true
				before := len(src.Groups)
				override.Sources.Gitlab[i].Groups = mergeStrings(src.Groups, groups)
				if tokenEnv != "" {
					override.Sources.Gitlab[i].TokenEnv = tokenEnv
					override.Sources.Gitlab[i].TokenCmd = ""
				} else if tokenCmd != "" {
					override.Sources.Gitlab[i].TokenCmd = tokenCmd
					override.Sources.Gitlab[i].TokenEnv = ""
				}
				added = len(override.Sources.Gitlab[i].Groups) - before
				break
			}
			if !found {
				override.Sources.Gitlab = append(override.Sources.Gitlab, config.GitlabSource{
					Host:     host,
					TokenEnv: tokenEnv,
					TokenCmd: tokenCmd,
					Groups:   groups,
				})
				added = len(groups)
			}

			if err := config.SaveOverrideConfig(globalCwd, override); err != nil {
				return fmt.Errorf("failed to save local override: %w", err)
			}

			if added == 0 {
				fmt.Printf("All groups already configured for %s\n", host)
			} else {
				fmt.Printf("Added %d GitLab group(s) under %s\n", added, host)
			}
			if tokenEnv == "" && tokenCmd == "" {
				fmt.Printf("Token: using system keyring for %s. Run 'gitte token set gitlab %s' if not already stored.\n", host, host)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenEnv, "token-env", "", "env var containing the API token")
	cmd.Flags().StringVar(&tokenCmd, "token-cmd", "", "shell command to retrieve the API token")
	return cmd
}

func newSourcesAddGithubCmd() *cobra.Command {
	var tokenEnv, tokenCmd string
	cmd := &cobra.Command{
		Use:   "github <host> <org> [org...]",
		Short: "Add GitHub orgs to local discovery sources",
		Long: `Add one or more GitHub orgs to local discovery sources.

By default the token is looked up from the system keyring. Run:
  gitte token set github <host>

to store a token with read:org scope. Public orgs work without a token.

Examples:
  gitte sources add github github.com myorg
  gitte sources add github github.com orgA orgB
  gitte sources add github github.com myorg --token-env MY_GITHUB_TOKEN
  gitte sources add github github.com myorg --token-cmd "pass show github/token"`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			orgs := args[1:]

			override, err := config.LoadOverrideConfig(globalCwd)
			if err != nil {
				return fmt.Errorf("failed to load local override: %w", err)
			}

			added := 0
			found := false
			for i, src := range override.Sources.Github {
				if src.Host != host {
					continue
				}
				found = true
				before := len(src.Orgs)
				override.Sources.Github[i].Orgs = mergeStrings(src.Orgs, orgs)
				if tokenEnv != "" {
					override.Sources.Github[i].TokenEnv = tokenEnv
					override.Sources.Github[i].TokenCmd = ""
				} else if tokenCmd != "" {
					override.Sources.Github[i].TokenCmd = tokenCmd
					override.Sources.Github[i].TokenEnv = ""
				}
				added = len(override.Sources.Github[i].Orgs) - before
				break
			}
			if !found {
				override.Sources.Github = append(override.Sources.Github, config.GithubSource{
					Host:     host,
					TokenEnv: tokenEnv,
					TokenCmd: tokenCmd,
					Orgs:     orgs,
				})
				added = len(orgs)
			}

			if err := config.SaveOverrideConfig(globalCwd, override); err != nil {
				return fmt.Errorf("failed to save local override: %w", err)
			}

			if added == 0 {
				fmt.Printf("All orgs already configured for %s\n", host)
			} else {
				fmt.Printf("Added %d GitHub org(s) under %s\n", added, host)
			}
			if tokenEnv == "" && tokenCmd == "" {
				fmt.Printf("Token: using system keyring for %s. Run 'gitte token set github %s' if not already stored.\n", host, host)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenEnv, "token-env", "", "env var containing the API token")
	cmd.Flags().StringVar(&tokenCmd, "token-cmd", "", "shell command to retrieve the API token")
	return cmd
}

func newSourcesRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove groups or orgs from local discovery sources",
	}
	cmd.AddCommand(
		newSourcesRemoveGitlabCmd(),
		newSourcesRemoveGithubCmd(),
	)
	return cmd
}

func newSourcesRemoveGitlabCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gitlab <host> [group...]",
		Short: "Remove GitLab groups from local discovery sources (omit groups to remove the entire host)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			groups := args[1:]

			override, err := config.LoadOverrideConfig(globalCwd)
			if err != nil {
				return fmt.Errorf("failed to load local override: %w", err)
			}

			var next []config.GitlabSource
			removed := 0
			for _, src := range override.Sources.Gitlab {
				if src.Host != host {
					next = append(next, src)
					continue
				}
				if len(groups) == 0 {
					removed += len(src.Groups)
					continue // drop entire entry
				}
				before := len(src.Groups)
				src.Groups = removeStrings(src.Groups, groups)
				removed += before - len(src.Groups)
				if len(src.Groups) > 0 {
					next = append(next, src)
				}
			}
			override.Sources.Gitlab = next

			if err := config.SaveOverrideConfig(globalCwd, override); err != nil {
				return fmt.Errorf("failed to save local override: %w", err)
			}

			if removed == 0 {
				fmt.Println("No matching entries found in local override")
			} else {
				fmt.Printf("Removed %d GitLab group(s) from %s\n", removed, host)
			}
			return nil
		},
	}
}

func newSourcesRemoveGithubCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "github <host> [org...]",
		Short: "Remove GitHub orgs from local discovery sources (omit orgs to remove the entire host)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			orgs := args[1:]

			override, err := config.LoadOverrideConfig(globalCwd)
			if err != nil {
				return fmt.Errorf("failed to load local override: %w", err)
			}

			var next []config.GithubSource
			removed := 0
			for _, src := range override.Sources.Github {
				if src.Host != host {
					next = append(next, src)
					continue
				}
				if len(orgs) == 0 {
					removed += len(src.Orgs)
					continue // drop entire entry
				}
				before := len(src.Orgs)
				src.Orgs = removeStrings(src.Orgs, orgs)
				removed += before - len(src.Orgs)
				if len(src.Orgs) > 0 {
					next = append(next, src)
				}
			}
			override.Sources.Github = next

			if err := config.SaveOverrideConfig(globalCwd, override); err != nil {
				return fmt.Errorf("failed to save local override: %w", err)
			}

			if removed == 0 {
				fmt.Println("No matching entries found in local override")
			} else {
				fmt.Printf("Removed %d GitHub org(s) from %s\n", removed, host)
			}
			return nil
		},
	}
}

// tokenLabel returns a short display label describing how the token is sourced.
func tokenLabel(tokenEnv, tokenCmd string) string {
	if tokenCmd != "" {
		cmd := tokenCmd
		if len(cmd) > 30 {
			cmd = cmd[:27] + "..."
		}
		return "cmd:" + cmd
	}
	if tokenEnv != "" {
		return "env:" + tokenEnv
	}
	return "keyring"
}

// mergeStrings appends items from add into base, skipping duplicates.
func mergeStrings(base, add []string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[s] = true
	}
	result := append([]string{}, base...)
	for _, s := range add {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}

// removeStrings returns base with all items from remove omitted.
func removeStrings(base, remove []string) []string {
	drop := make(map[string]bool, len(remove))
	for _, s := range remove {
		drop[s] = true
	}
	var result []string
	for _, s := range base {
		if !drop[s] {
			result = append(result, s)
		}
	}
	return result
}
