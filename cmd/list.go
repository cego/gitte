package cmd

import (
	"fmt"
	"sort"
	"strings"

	"gitte/config"
	"gitte/output"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var showAll bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects and their actions",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList(showAll)
		},
	}
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "include disabled projects")
	return cmd
}

type listProj struct {
	name     string // config key
	host     string
	pathSegs []string
	leaf     string
	actions  map[string]config.ProjectAction
	enabled  bool
}

func runList(showAll bool) error {
	plain := outputMode() == output.ModePlain

	// Which projects survive toggle filtering (enabled).
	enabledSet := make(map[string]bool, len(globalCfg.Projects))
	for k := range globalCfg.Projects {
		enabledSet[k] = true
	}

	// Build from the raw (unfiltered) config so disabled projects are visible.
	projects := make([]listProj, 0, len(globalRawCfg.Projects))
	for name, pc := range globalRawCfg.Projects {
		host, path, _, err := config.ParseRemoteURL(pc.Remote)
		if err != nil {
			host = "unknown"
			path = name
		}
		segs := strings.Split(path, "/")
		var pathSegs []string
		leaf := path
		if len(segs) > 1 {
			pathSegs = segs[:len(segs)-1]
			leaf = segs[len(segs)-1]
		}
		projects = append(projects, listProj{
			name:     name,
			host:     host,
			pathSegs: pathSegs,
			leaf:     leaf,
			actions:  pc.Actions,
			enabled:  enabledSet[name],
		})
	}

	// Styles — all return plain text when not in TTY mode.
	hostSty := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	nsSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	projSty := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	disabledSty := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	actionSty := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	countSty := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keySty := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	sty := func(s lipgloss.Style, text string) string {
		if plain {
			return text
		}
		return s.Render(text)
	}

	// Counts for the summary line.
	totalEnabled, totalDisabled := 0, 0
	for _, p := range projects {
		if p.enabled {
			totalEnabled++
		} else {
			totalDisabled++
		}
	}

	// Recursive namespace tree printer — mirrors flattenNS in the actions TUI:
	// namespaces (folders) first sorted, then leaf projects sorted.
	var printNS func(nodes []listProj, depth int)
	printNS = func(nodes []listProj, depth int) {
		indent := strings.Repeat("  ", depth)

		nsMap := make(map[string][]listProj)
		leafMap := make(map[string][]listProj)

		for _, p := range nodes {
			if len(p.pathSegs) == 0 {
				leafMap[p.leaf] = append(leafMap[p.leaf], p)
			} else {
				ns := p.pathSegs[0]
				pp := p
				pp.pathSegs = p.pathSegs[1:]
				nsMap[ns] = append(nsMap[ns], pp)
			}
		}

		nsKeys := sortedStringKeys(nsMap)
		leafKeys := make([]string, 0, len(leafMap))
		for k := range leafMap {
			if _, isNS := nsMap[k]; !isNS {
				leafKeys = append(leafKeys, k)
			}
		}
		sort.Strings(leafKeys)

		// Namespaces first.
		for _, ns := range nsKeys {
			fmt.Println(indent + sty(nsSty, ns))
			printNS(nsMap[ns], depth+1)
		}

		// Leaf projects: enabled first, then (optionally) disabled.
		var disabledLeaves []listProj
		disabledCount := 0

		for _, leaf := range leafKeys {
			for _, p := range leafMap[leaf] {
				if !p.enabled {
					disabledCount++
					if showAll {
						disabledLeaves = append(disabledLeaves, p)
					}
					continue
				}
				label := p.leaf
				if p.name != p.leaf {
					label += " " + sty(keySty, "("+p.name+")")
				}
				actStr := buildActionStr(p.actions, plain, sty, actionSty)
				fmt.Printf("%s%s  %s\n", indent, sty(projSty, label), actStr)
			}
		}

		if showAll {
			for _, p := range disabledLeaves {
				label := p.leaf
				if p.name != p.leaf {
					label += " " + sty(keySty, "("+p.name+")")
				}
				actStr := buildActionStr(p.actions, plain, sty, actionSty)
				if actStr != "" {
					fmt.Printf("%s%s  %s\n", indent, sty(disabledSty, label), actStr)
				} else {
					fmt.Printf("%s%s\n", indent, sty(disabledSty, label))
				}
			}
		} else if disabledCount > 0 {
			noun := "project"
			if disabledCount > 1 {
				noun = "projects"
			}
			fmt.Printf("%s%s\n", indent, sty(countSty, fmt.Sprintf("(+%d disabled %s)", disabledCount, noun)))
		}
	}

	// Group by host then print each host subtree.
	hostMap := make(map[string][]listProj)
	for _, p := range projects {
		hostMap[p.host] = append(hostMap[p.host], p)
	}

	for _, host := range sortedStringKeys(hostMap) {
		fmt.Println(sty(hostSty, host))
		printNS(hostMap[host], 1)
	}

	// Summary line.
	summary := fmt.Sprintf("%d enabled", totalEnabled)
	if totalDisabled > 0 && !showAll {
		summary += fmt.Sprintf(", %d disabled (use --all to show)", totalDisabled)
	} else if totalDisabled > 0 {
		summary += fmt.Sprintf(", %d disabled", totalDisabled)
	}
	fmt.Println()
	fmt.Println(sty(countSty, summary))

	return nil
}

// buildActionStr returns a sorted, comma-joined list of action names.
func buildActionStr(actions map[string]config.ProjectAction, plain bool, sty func(lipgloss.Style, string) string, actionSty lipgloss.Style) string {
	names := sortedStringKeys(actions)
	return sty(actionSty, strings.Join(names, ","))
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
