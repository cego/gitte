package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cego/gitte/output"
	"github.com/cego/gitte/state"
	"github.com/cego/gitte/toggle"

	"github.com/spf13/cobra"
)

func newToggleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toggle [projects...]",
		Short: "Enable/disable projects (TUI or CLI)",
		Long:  "Without arguments, launches an interactive TUI. With project names, toggles them directly.",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return allProjectNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return runToggleCLI(args)
			}
			if outputMode() == output.ModePlain {
				return runToggleList()
			}
			return toggle.Run(globalRawCfg, globalCwd, globalSt)
		},
	}

	cmd.AddCommand(newToggleListCmd(), newToggleEnableCmd(), newToggleDisableCmd())

	return cmd
}

func newToggleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects and their toggle state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToggleList()
		},
	}
}

func newToggleEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <projects...>",
		Short: "Enable specific projects",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return allProjectNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return setToggleState(args, true)
		},
	}
}

func newToggleDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <projects...>",
		Short: "Disable specific projects",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return allProjectNames(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return setToggleState(args, false)
		},
	}
}

func runToggleCLI(args []string) error {
	for _, name := range args {
		if _, ok := globalRawCfg.Projects[name]; !ok {
			return fmt.Errorf("unknown project: %q", name)
		}
	}
	for _, name := range args {
		proj := globalRawCfg.Projects[name]
		defaultState := !proj.DefaultDisabled
		current, exists := globalSt.Toggles[name]
		if !exists {
			current = defaultState
		}
		newState := !current
		if newState == defaultState {
			delete(globalSt.Toggles, name)
		} else {
			globalSt.Toggles[name] = newState
		}
		label := "enabled"
		if !newState {
			label = "disabled"
		}
		fmt.Printf("%s: %s\n", name, label)
	}
	return state.Save(globalCwd, globalSt)
}

func setToggleState(args []string, enabled bool) error {
	for _, name := range args {
		if _, ok := globalRawCfg.Projects[name]; !ok {
			return fmt.Errorf("unknown project: %q", name)
		}
	}
	for _, name := range args {
		proj := globalRawCfg.Projects[name]
		defaultState := !proj.DefaultDisabled
		if enabled == defaultState {
			delete(globalSt.Toggles, name)
		} else {
			globalSt.Toggles[name] = enabled
		}
		label := "enabled"
		if !enabled {
			label = "disabled"
		}
		fmt.Printf("%s: %s\n", name, label)
	}
	return state.Save(globalCwd, globalSt)
}

func runToggleList() error {
	names := make([]string, 0, len(globalRawCfg.Projects))
	for name := range globalRawCfg.Projects {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("%-40s %s\n", "PROJECT", "STATE")
	fmt.Printf("%-40s %s\n", strings.Repeat("-", 40), "-----")
	for _, name := range names {
		proj := globalRawCfg.Projects[name]
		defaultState := !proj.DefaultDisabled
		current, exists := globalSt.Toggles[name]
		if !exists {
			current = defaultState
		}
		stateStr := "enabled"
		if !current {
			stateStr = "disabled"
		}
		if current != defaultState {
			stateStr += " (custom)"
		}
		fmt.Printf("%-40s %s\n", name, stateStr)
	}
	return nil
}

// allProjectNames returns all project keys from the raw (unfiltered) config.
func allProjectNames() []string {
	if globalRawCfg == nil {
		return nil
	}
	out := make([]string, 0, len(globalRawCfg.Projects))
	for name := range globalRawCfg.Projects {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
