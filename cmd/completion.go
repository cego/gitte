package cmd

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// completeWithPlus handles completion for arguments that support the "a+b+c" join syntax.
// toComplete is the partial word being typed; candidates is the full set of valid values.
func completeWithPlus(toComplete string, candidates []string) []string {
	parts := strings.Split(toComplete, "+")
	suffix := parts[len(parts)-1]
	prefix := strings.Join(parts[:len(parts)-1], "+")
	if prefix != "" {
		prefix += "+"
	}

	// Deduplicate: don't offer values already present before the last +.
	used := make(map[string]bool, len(parts)-1)
	for _, p := range parts[:len(parts)-1] {
		used[p] = true
	}

	var out []string
	for _, c := range candidates {
		if !used[c] && strings.HasPrefix(c, suffix) {
			out = append(out, prefix+c)
		}
	}
	sort.Strings(out)
	return out
}

// actionNames returns all unique action names across all enabled projects.
func actionNames() []string {
	if globalCfg == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, proj := range globalCfg.Projects {
		for a := range proj.Actions {
			seen[a] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// projectNames returns all enabled project keys.
func projectNames() []string {
	if globalCfg == nil {
		return nil
	}
	out := make([]string, 0, len(globalCfg.Projects))
	for name := range globalCfg.Projects {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// groupNames returns all group names: keys from groupIncludes plus inline groups from actions.
func groupNames() []string {
	if globalCfg == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for g := range globalCfg.GroupIncludes {
		seen[g] = struct{}{}
	}
	for _, proj := range globalCfg.Projects {
		for _, action := range proj.Actions {
			for g := range action.Groups {
				seen[g] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// actionArgsCompletion is the ValidArgsFunction for commands with the
// <action> [group] [projects] positional argument convention.
func actionArgsCompletion(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeWithPlus(toComplete, actionNames()), cobra.ShellCompDirectiveNoFileComp
	case 1:
		return completeWithPlus(toComplete, groupNames()), cobra.ShellCompDirectiveNoFileComp
	case 2:
		return completeWithPlus(toComplete, projectNames()), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}
