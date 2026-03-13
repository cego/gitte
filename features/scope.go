package features

import (
	"sort"
	"strings"

	"github.com/cego/gitte/state"
)

// ProjectMatchesOverrideScope checks if a project is included in an override scope.
// projName is the config key, host and path come from config.ParseRemoteURL.
// Returns false if override is nil (caller should use config scope instead).
func ProjectMatchesOverrideScope(projName, host, path string, override *state.ScopeOverride) bool {
	if override == nil {
		return false
	}

	for _, p := range override.Projects {
		if p == projName {
			return true
		}
	}

	for _, gs := range override.GitlabGroups {
		if gs.Host != host {
			continue
		}
		if path == gs.Group || strings.HasPrefix(path, gs.Group+"/") {
			if !containsString(gs.ExcludeProjects, projName) {
				return true
			}
		}
	}

	for _, ghs := range override.GithubOrgs {
		if ghs.Host != host {
			continue
		}
		if strings.HasPrefix(path, ghs.Org+"/") {
			if !containsString(ghs.ExcludeProjects, projName) {
				return true
			}
		}
	}

	return false
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ScopeProject holds parsed remote info for a project within a gate's scope.
type ScopeProject struct {
	Host string
	Path string // full path from ParseRemoteURL, e.g. "cego/monolith"
}

// ScopeRowKind distinguishes tree node types.
type ScopeRowKind int

const (
	ScopeRowHost      ScopeRowKind = iota // e.g. gitlab.cego.dk
	ScopeRowNamespace                     // e.g. cego, services
	ScopeRowProject                       // leaf project
)

// ScopeRow is a flat row in the scope tree.
type ScopeRow struct {
	Kind     ScopeRowKind
	Label    string
	Depth    int
	ProjName string   // config key, only set for ScopeRowProject
	Children []string // config keys of all leaf projects under this node (for branches)
}

// BuildScopeTree builds a flat row list grouped by host → namespace segments → project.
func BuildScopeTree(projects map[string]ScopeProject) []ScopeRow {
	hostMap := make(map[string]map[string]ScopeProject)
	for name, sp := range projects {
		if hostMap[sp.Host] == nil {
			hostMap[sp.Host] = make(map[string]ScopeProject)
		}
		hostMap[sp.Host][name] = sp
	}

	hosts := make([]string, 0, len(hostMap))
	for h := range hostMap {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)

	var rows []ScopeRow
	for _, host := range hosts {
		hostProjs := hostMap[host]
		hostChildren := sortedKeys(hostProjs)
		rows = append(rows, ScopeRow{Kind: ScopeRowHost, Label: host, Depth: 0, Children: hostChildren})
		rows = append(rows, buildNamespaceRows(hostProjs, 1)...)
	}
	return rows
}

func buildNamespaceRows(projects map[string]ScopeProject, depth int) []ScopeRow {
	type nsEntry struct {
		name string
		sp   ScopeProject
	}

	nsMap := make(map[string][]nsEntry)
	var leafs []nsEntry

	for name, sp := range projects {
		parts := strings.SplitN(sp.Path, "/", 2)
		if len(parts) == 1 {
			leafs = append(leafs, nsEntry{name: name, sp: sp})
		} else {
			seg := parts[0]
			nsMap[seg] = append(nsMap[seg], nsEntry{
				name: name,
				sp:   ScopeProject{Host: sp.Host, Path: parts[1]},
			})
		}
	}

	nsKeys := make([]string, 0, len(nsMap))
	for k := range nsMap {
		nsKeys = append(nsKeys, k)
	}
	sort.Strings(nsKeys)
	sort.Slice(leafs, func(i, j int) bool { return leafs[i].name < leafs[j].name })

	var rows []ScopeRow

	for _, l := range leafs {
		rows = append(rows, ScopeRow{
			Kind: ScopeRowProject, Label: l.name, Depth: depth, ProjName: l.name,
		})
	}

	for _, seg := range nsKeys {
		entries := nsMap[seg]
		children := make([]string, len(entries))
		subProjs := make(map[string]ScopeProject, len(entries))
		for i, e := range entries {
			children[i] = e.name
			subProjs[e.name] = e.sp
		}
		sort.Strings(children)
		rows = append(rows, ScopeRow{
			Kind: ScopeRowNamespace, Label: seg, Depth: depth, Children: children,
		})
		rows = append(rows, buildNamespaceRows(subProjs, depth+1)...)
	}

	return rows
}

func sortedKeys(m map[string]ScopeProject) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CheckedStateToOverride converts a map of project checked states to a ScopeOverride.
// Returns nil if all projects are checked (full scope). Returns empty override if none checked.
func CheckedStateToOverride(checked map[string]bool, projects map[string]ScopeProject) *state.ScopeOverride {
	allChecked := true
	anyChecked := false
	for _, v := range checked {
		if !v {
			allChecked = false
		} else {
			anyChecked = true
		}
	}
	if allChecked {
		return nil
	}
	if !anyChecked {
		return &state.ScopeOverride{}
	}

	type groupInfo struct {
		host     string
		segment  string
		projects map[string]bool
	}

	groups := make(map[string]*groupInfo)
	var standaloneProjects []string

	for name, sp := range projects {
		parts := strings.SplitN(sp.Path, "/", 2)
		if len(parts) == 1 {
			if checked[name] {
				standaloneProjects = append(standaloneProjects, name)
			}
			continue
		}
		key := sp.Host + "/" + parts[0]
		if groups[key] == nil {
			groups[key] = &groupInfo{
				host:     sp.Host,
				segment:  parts[0],
				projects: make(map[string]bool),
			}
		}
		groups[key].projects[name] = checked[name]
	}

	override := &state.ScopeOverride{}

	for _, gi := range groups {
		allGroupChecked := true
		anyGroupChecked := false
		var excluded []string
		for name, isChecked := range gi.projects {
			if !isChecked {
				allGroupChecked = false
				excluded = append(excluded, name)
			} else {
				anyGroupChecked = true
			}
		}

		if !anyGroupChecked {
			continue
		}

		sort.Strings(excluded)

		isGithub := strings.Contains(gi.host, "github")
		if isGithub {
			entry := state.ScopeOverrideOrg{Host: gi.host, Org: gi.segment}
			if !allGroupChecked {
				entry.ExcludeProjects = excluded
			}
			override.GithubOrgs = append(override.GithubOrgs, entry)
		} else {
			entry := state.ScopeOverrideGroup{Host: gi.host, Group: gi.segment}
			if !allGroupChecked {
				entry.ExcludeProjects = excluded
			}
			override.GitlabGroups = append(override.GitlabGroups, entry)
		}
	}

	sort.Strings(standaloneProjects)
	override.Projects = standaloneProjects

	return override
}

// OverrideToCheckedState converts a ScopeOverride to a per-project checked map.
// If override is nil, all projects are checked (full scope).
func OverrideToCheckedState(override *state.ScopeOverride, projects map[string]ScopeProject) map[string]bool {
	checked := make(map[string]bool, len(projects))
	if override == nil {
		for name := range projects {
			checked[name] = true
		}
		return checked
	}

	for name, sp := range projects {
		checked[name] = ProjectMatchesOverrideScope(name, sp.Host, sp.Path, override)
	}
	return checked
}
