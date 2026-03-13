# Feature Gate TUI Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Interactive BubbleTea TUI for toggling feature gates and customizing per-project scope, with extended CLI flags for non-TTY use.

**Architecture:** Two-screen BubbleTea model (gate list → scope tree) in a new `features/` package. State model extended with structural scope overrides (gitlab groups, github orgs, exclude lists). Scope resolution updated in `actions/runner.go`.

**Tech Stack:** Go, BubbleTea, lipgloss, cobra (existing dependencies)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `state/state.go` | Modify | Extend `ScopeOverride` with `GitlabGroups`, `GithubOrgs`, sub-types |
| `features/scope.go` | Create | Scope tree building, checked↔ScopeOverride conversion, override scope matching |
| `features/scope_test.go` | Create | Tests for scope logic |
| `features/tui.go` | Create | BubbleTea model, rendering, key handling for both screens |
| `actions/runner.go` | Modify | Use new override scope matching, fix GitLab `/` delimiter |
| `actions/runner_test.go` | Modify | Update tests for new scope matching behavior |
| `cmd/features.go` | Modify | Add TUI launch, extend enable flags |

---

## Chunk 1: State Model and Scope Logic

### Task 1: Extend ScopeOverride types

**Files:**
- Modify: `state/state.go:29-32`

- [ ] **Step 1: Add new types to state/state.go**

Replace the existing `ScopeOverride` struct and add the two new sub-types:

```go
// ScopeOverride narrows a feature gate's configured scope on this machine
type ScopeOverride struct {
	Projects     []string              `yaml:"projects,omitempty"`
	GitlabGroups []ScopeOverrideGroup  `yaml:"gitlab_groups,omitempty"`
	GithubOrgs   []ScopeOverrideOrg    `yaml:"github_orgs,omitempty"`
}

// ScopeOverrideGroup scopes a feature gate to a GitLab group with optional exclusions
type ScopeOverrideGroup struct {
	Host            string   `yaml:"host"`
	Group           string   `yaml:"group"`
	ExcludeProjects []string `yaml:"exclude_projects,omitempty"`
}

// ScopeOverrideOrg scopes a feature gate to a GitHub org with optional exclusions
type ScopeOverrideOrg struct {
	Host            string   `yaml:"host"`
	Org             string   `yaml:"org"`
	ExcludeProjects []string `yaml:"exclude_projects,omitempty"`
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: compiles cleanly (existing code that sets `ScopeOverride{Projects: ...}` still works)

- [ ] **Step 3: Commit**

```bash
git add state/state.go
git commit -m "Extend ScopeOverride with GitlabGroups and GithubOrgs"
```

---

### Task 2: Scope matching with override support

**Files:**
- Create: `features/scope.go`
- Create: `features/scope_test.go`

- [ ] **Step 1: Write tests for ProjectMatchesOverrideScope**

Create `features/scope_test.go`:

```go
package features

import (
	"testing"

	"github.com/cego/gitte/state"
)

func TestProjectMatchesOverrideScope_NilOverride(t *testing.T) {
	// nil override means "use full config scope" — caller handles this,
	// so this function should not be called with nil. Test that it returns false.
	if ProjectMatchesOverrideScope("proj", "gitlab.cego.dk", "cego/monolith", nil) {
		t.Error("expected false for nil override")
	}
}

func TestProjectMatchesOverrideScope_ExplicitProjects(t *testing.T) {
	override := &state.ScopeOverride{
		Projects: []string{"monolith", "mysql"},
	}
	if !ProjectMatchesOverrideScope("monolith", "gitlab.cego.dk", "cego/monolith", override) {
		t.Error("expected monolith to match")
	}
	if ProjectMatchesOverrideScope("redis", "gitlab.cego.dk", "cego/redis", override) {
		t.Error("expected redis to not match")
	}
}

func TestProjectMatchesOverrideScope_GitlabGroup(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "cego"},
		},
	}
	if !ProjectMatchesOverrideScope("monolith", "gitlab.cego.dk", "cego/monolith", override) {
		t.Error("expected cego/monolith to match group cego")
	}
	if ProjectMatchesOverrideScope("promo", "gitlab.cego.dk", "spilnu/promo", override) {
		t.Error("expected spilnu/promo to not match group cego")
	}
}

func TestProjectMatchesOverrideScope_GitlabGroupDelimiter(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "spilnu"},
		},
	}
	// "spilnu-legacy/foo" should NOT match group "spilnu" (delimiter fix)
	if ProjectMatchesOverrideScope("foo", "gitlab.cego.dk", "spilnu-legacy/foo", override) {
		t.Error("expected spilnu-legacy/foo to not match group spilnu")
	}
	// "spilnu/services/promo" should match group "spilnu"
	if !ProjectMatchesOverrideScope("promo", "gitlab.cego.dk", "spilnu/services/promo", override) {
		t.Error("expected spilnu/services/promo to match group spilnu")
	}
}

func TestProjectMatchesOverrideScope_ExcludeProjects(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "cego", ExcludeProjects: []string{"mysql"}},
		},
	}
	if !ProjectMatchesOverrideScope("monolith", "gitlab.cego.dk", "cego/monolith", override) {
		t.Error("expected monolith to match (not excluded)")
	}
	if ProjectMatchesOverrideScope("mysql", "gitlab.cego.dk", "cego/mysql", override) {
		t.Error("expected mysql to be excluded")
	}
}

func TestProjectMatchesOverrideScope_GithubOrg(t *testing.T) {
	override := &state.ScopeOverride{
		GithubOrgs: []state.ScopeOverrideOrg{
			{Host: "github.com", Org: "cego"},
		},
	}
	if !ProjectMatchesOverrideScope("gitte", "github.com", "cego/gitte", override) {
		t.Error("expected cego/gitte to match org cego")
	}
	if ProjectMatchesOverrideScope("other", "github.com", "other-org/repo", override) {
		t.Error("expected other-org/repo to not match org cego")
	}
}

func TestProjectMatchesOverrideScope_EmptyOverride(t *testing.T) {
	// All fields empty = nothing matches (gate effectively disabled)
	override := &state.ScopeOverride{}
	if ProjectMatchesOverrideScope("proj", "gitlab.cego.dk", "cego/proj", override) {
		t.Error("expected false for empty override")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./features/`
Expected: FAIL — `features/scope.go` does not exist yet

- [ ] **Step 3: Implement ProjectMatchesOverrideScope**

Create `features/scope.go`:

```go
package features

import (
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./features/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add features/scope.go features/scope_test.go
git commit -m "Add ProjectMatchesOverrideScope with exclude list support"
```

---

### Task 3: Update actions/runner.go to use new scope resolution

**Files:**
- Modify: `actions/runner.go:295-398`

- [ ] **Step 1: Fix GitLab group prefix matching delimiter**

In `actions/runner.go`, function `projectMatchesScope` (around line 373), change:

```go
if gs.Host == host && strings.HasPrefix(path, gs.Group) {
```

to:

```go
if gs.Host == host && (path == gs.Group || strings.HasPrefix(path, gs.Group+"/")) {
```

- [ ] **Step 2: Update extraEnvForProject to use override scope**

In `actions/runner.go`, function `extraEnvForProject` (around line 308-313), replace the override scope conversion block:

```go
		scope := gate.Scope
		if fs.OverrideScope != nil {
			scope = config.FeatureScope{
				Projects: fs.OverrideScope.Projects,
			}
		}

		if projectMatchesScope(proj, scope) {
```

with:

```go
		if fs.OverrideScope != nil {
			host, path, _, err := config.ParseRemoteURL(proj.Remote)
			if err != nil {
				continue
			}
			if !features.ProjectMatchesOverrideScope(projName, host, path, fs.OverrideScope) {
				continue
			}
		} else if !projectMatchesScope(proj, gate.Scope) {
			continue
		}
```

Note: `extraEnvForProject` currently doesn't receive `projName`. You need to update its signature to accept it, and update the two call sites in `runner.go` (`buildTaskInfos` and `buildEnv`). In `buildTaskInfos`, the project name is `key.Project`. In `buildEnv`, add a `projName string` parameter and pass it from `runGroupTask`.

Add the import: `"github.com/cego/gitte/features"`

- [ ] **Step 3: Update function signatures**

Change `extraEnvForProject` signature from:

```go
func extraEnvForProject(cfg *config.GitteConfig, st *state.GitteState, proj config.ProjectConfig) map[string]string {
```

to:

```go
func extraEnvForProject(cfg *config.GitteConfig, st *state.GitteState, projName string, proj config.ProjectConfig) map[string]string {
```

Update call in `buildTaskInfos` (around line 166):
```go
for k, v := range extraEnvForProject(cfg, st, key.Project, proj) {
```

Update call in `buildEnv` — change signature to also accept `projName string` and pass it through:
```go
func buildEnv(cfg *config.GitteConfig, st *state.GitteState, projName string, proj config.ProjectConfig) []string {
```
```go
featureEnv := extraEnvForProject(cfg, st, projName, proj)
```

Update call to `buildEnv` in `runGroupTask` (around line 270):
```go
env := buildEnv(cfg, st, taskName, proj)
```

Wait — `taskName` here is `project:action:group`, not the project name. Instead, the project name should be extracted. Look at `buildExecutorTasks`: the closure captures `key.Project`. Pass it into `runGroupTask`:

Update `runGroupTask` signature to add `projName string`, update the closure in `buildExecutorTasks` to pass `key.Project`, and use it in `buildEnv`.

- [ ] **Step 4: Update ProjectMatchesScopeByName similarly**

In `ProjectMatchesScopeByName` (around line 388), the function is used elsewhere. It should also support override scope, but since it takes a `config.FeatureScope` (not an override), leave it as-is for now. The override logic is handled in `extraEnvForProject`.

- [ ] **Step 5: Build and run tests**

Run: `go build ./... && go test ./...`
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add actions/runner.go features/scope.go
git commit -m "Use override scope matching with exclude support in env resolution"
```

---

### Task 4: Scope tree building and checked-state conversion

**Files:**
- Modify: `features/scope.go`
- Modify: `features/scope_test.go`

- [ ] **Step 1: Write tests for BuildScopeTree**

Append to `features/scope_test.go`:

```go
func TestBuildScopeTree_GroupsProjectsByHostAndNamespace(t *testing.T) {
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
		"promo":    {Host: "gitlab.cego.dk", Path: "spilnu/services/promo"},
	}
	rows := BuildScopeTree(projects)

	// Expect: host row, namespace rows, project rows
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	// First row should be host
	if rows[0].Kind != ScopeRowHost {
		t.Errorf("expected first row to be host, got %v", rows[0].Kind)
	}
	if rows[0].Label != "gitlab.cego.dk" {
		t.Errorf("expected label gitlab.cego.dk, got %q", rows[0].Label)
	}
}

func TestCheckedStateToOverride_AllChecked(t *testing.T) {
	checked := map[string]bool{"monolith": true, "mysql": true}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	override := CheckedStateToOverride(checked, projects)
	if override != nil {
		t.Error("expected nil override when all checked")
	}
}

func TestCheckedStateToOverride_NoneChecked(t *testing.T) {
	checked := map[string]bool{"monolith": false, "mysql": false}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	override := CheckedStateToOverride(checked, projects)
	// Empty override = disabled (caller interprets this)
	if override == nil {
		t.Fatal("expected non-nil override when none checked")
	}
	if len(override.Projects) != 0 && len(override.GitlabGroups) != 0 && len(override.GithubOrgs) != 0 {
		t.Error("expected empty override")
	}
}

func TestCheckedStateToOverride_PartialGroup(t *testing.T) {
	checked := map[string]bool{"monolith": true, "mysql": false}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	override := CheckedStateToOverride(checked, projects)
	if override == nil {
		t.Fatal("expected non-nil override")
	}
	// Should have gitlab group "cego" with mysql excluded
	if len(override.GitlabGroups) != 1 {
		t.Fatalf("expected 1 gitlab group, got %d", len(override.GitlabGroups))
	}
	g := override.GitlabGroups[0]
	if g.Group != "cego" {
		t.Errorf("expected group cego, got %q", g.Group)
	}
	if len(g.ExcludeProjects) != 1 || g.ExcludeProjects[0] != "mysql" {
		t.Errorf("expected exclude [mysql], got %v", g.ExcludeProjects)
	}
}

func TestOverrideToCheckedState(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "cego", ExcludeProjects: []string{"mysql"}},
		},
	}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	checked := OverrideToCheckedState(override, projects)
	if !checked["monolith"] {
		t.Error("expected monolith checked")
	}
	if checked["mysql"] {
		t.Error("expected mysql unchecked")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./features/`
Expected: FAIL — types and functions don't exist

- [ ] **Step 3: Implement scope tree types and BuildScopeTree**

Add to `features/scope.go`:

```go
import (
	"sort"
	// ... existing imports
)

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
	// Group by host.
	hostMap := make(map[string]map[string]ScopeProject) // host → (projName → ScopeProject)
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

// buildNamespaceRows recursively groups projects by their next path segment.
func buildNamespaceRows(projects map[string]ScopeProject, depth int) []ScopeRow {
	type nsEntry struct {
		name string
		sp   ScopeProject
		rest string // remaining path after this segment
	}

	nsMap := make(map[string][]nsEntry) // first segment → entries
	var leafs []nsEntry                 // projects with no more segments

	for name, sp := range projects {
		parts := strings.SplitN(sp.Path, "/", 2)
		if len(parts) == 1 {
			leafs = append(leafs, nsEntry{name: name, sp: sp})
		} else {
			seg := parts[0]
			nsMap[seg] = append(nsMap[seg], nsEntry{
				name: name,
				sp:   ScopeProject{Host: sp.Host, Path: parts[1]},
				rest: parts[1],
			})
		}
	}

	// Sort namespaces and leafs.
	nsKeys := make([]string, 0, len(nsMap))
	for k := range nsMap {
		nsKeys = append(nsKeys, k)
	}
	sort.Strings(nsKeys)
	sort.Slice(leafs, func(i, j int) bool { return leafs[i].name < leafs[j].name })

	var rows []ScopeRow

	// Leaf projects first (projects at this level).
	for _, l := range leafs {
		rows = append(rows, ScopeRow{
			Kind: ScopeRowProject, Label: l.name, Depth: depth, ProjName: l.name,
		})
	}

	// Namespace groups.
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
```

- [ ] **Step 4: Implement CheckedStateToOverride**

Add to `features/scope.go`:

```go
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
		return nil // full scope
	}
	if !anyChecked {
		return &state.ScopeOverride{} // empty = disabled
	}

	// Group projects by host+firstSegment to find groups/orgs.
	type groupInfo struct {
		host     string
		segment  string // first path segment (group/org)
		projects map[string]bool // projName → checked
	}

	groups := make(map[string]*groupInfo) // "host/segment" → info
	var standaloneProjects []string       // projects with no path separator

	for name, sp := range projects {
		parts := strings.SplitN(sp.Path, "/", 2)
		if len(parts) == 1 {
			// Root-level project, no group
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

	// Determine if host looks like github (heuristic: contains "github")
	// Otherwise assume gitlab.
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
			continue // entire group unchecked, omit
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
```

- [ ] **Step 5: Implement OverrideToCheckedState**

Add to `features/scope.go`:

```go
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
```

- [ ] **Step 6: Run tests**

Run: `go test ./features/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add features/scope.go features/scope_test.go
git commit -m "Add scope tree building and checked-state conversion"
```

---

## Chunk 2: TUI Implementation

### Task 5: Gate list screen (Screen 1)

**Files:**
- Create: `features/tui.go`

- [ ] **Step 1: Create features/tui.go with model types and gate list rendering**

```go
package features

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/state"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenGateList screen = iota
	screenScopeTree
)

// gateInfo holds display info for a single feature gate.
type gateInfo struct {
	Name        string
	Description string
	Gate        config.FeatureGate
}

type featuresModel struct {
	screen screen
	width  int
	height int
	cwd    string
	st     *state.GitteState
	cfg    *config.GitteConfig

	// Gate list state
	gates      []gateInfo
	gateCursor int

	// Scope tree state (populated when entering a gate)
	scopeGate    string // name of the gate being edited
	scopeRows    []ScopeRow
	scopeChecked map[string]bool    // projName → checked
	scopeProjects map[string]ScopeProject // projName → parsed info
	scopeCursor  int
	scopeOffset  int
	undoStack    []map[string]bool // each entry: projName → previous checked value
}

var (
	ftTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	ftHostStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	ftNsStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	ftSelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	ftCurStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	ftCheckStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	ftDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ftFailStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	ftHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func newFeaturesModel(cfg *config.GitteConfig, cwd string, st *state.GitteState) *featuresModel {
	gates := make([]gateInfo, 0, len(cfg.FeatureGates))
	for name, gate := range cfg.FeatureGates {
		gates = append(gates, gateInfo{Name: name, Description: gate.Description, Gate: gate})
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].Name < gates[j].Name })

	return &featuresModel{
		screen: screenGateList,
		cwd:    cwd,
		st:     st,
		cfg:    cfg,
		gates:  gates,
	}
}

func (m *featuresModel) Init() tea.Cmd { return nil }

// gateStatus returns the display status for a gate.
func (m *featuresModel) gateStatus(g gateInfo) string {
	fs, ok := m.st.Features[g.Name]
	if !ok || !fs.Enabled {
		return "[ ]"
	}
	if fs.OverrideScope == nil {
		return ftCheckStyle.Render("[✓]")
	}
	// Check if override is non-empty (narrowed)
	if len(fs.OverrideScope.Projects) > 0 || len(fs.OverrideScope.GitlabGroups) > 0 || len(fs.OverrideScope.GithubOrgs) > 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("[·]")
	}
	return "[ ]" // empty override = disabled
}

func (m *featuresModel) viewGateList() string {
	var b strings.Builder
	b.WriteString(ftTitleStyle.Render("Feature Gates"))
	b.WriteString("\n\n")

	if len(m.gates) == 0 {
		b.WriteString(ftDimStyle.Render("No feature gates defined in configuration."))
		b.WriteString("\n\n")
		b.WriteString(ftHelpStyle.Render("Press q to quit"))
		return b.String()
	}

	for i, g := range m.gates {
		cursor := "  "
		if i == m.gateCursor {
			cursor = ftCurStyle.Render("> ")
		}

		status := m.gateStatus(g)

		var nameStr string
		if i == m.gateCursor {
			nameStr = ftSelStyle.Render(g.Name)
		} else {
			nameStr = g.Name
		}

		desc := ""
		if g.Description != "" {
			desc = " — " + ftDimStyle.Render(g.Description)
		}

		b.WriteString(cursor + status + " " + nameStr + desc + "\n")
	}

	b.WriteString("\n")
	b.WriteString(ftHelpStyle.Render("↑↓/jk: nav  Space: toggle  Enter: edit scope  q/Esc: save & quit"))
	return b.String()
}
```

- [ ] **Step 2: Build to verify**

Run: `go build ./features/`
Expected: compiles

- [ ] **Step 3: Commit**

```bash
git add features/tui.go
git commit -m "Add features TUI model and gate list rendering"
```

---

### Task 6: Scope tree screen (Screen 2)

**Files:**
- Modify: `features/tui.go`

- [ ] **Step 1: Add scope tree rendering**

Add the `viewScopeTree` method and helper to `features/tui.go`:

```go
// branchState returns tri-state for a branch node: all checked, some, none.
func (m *featuresModel) branchState(children []string) string {
	all := true
	any := false
	for _, name := range children {
		if m.scopeChecked[name] {
			any = true
		} else {
			all = false
		}
	}
	if all {
		return ftCheckStyle.Render("[✓]")
	}
	if any {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("[·]")
	}
	return "[ ]"
}

func (m *featuresModel) viewScopeTree() string {
	var b strings.Builder

	// Header
	g := m.gates[m.gateCursor]
	desc := g.Description
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	b.WriteString(ftTitleStyle.Render("← " + g.Name))
	if desc != "" {
		b.WriteString(" — " + ftDimStyle.Render(desc))
	}
	b.WriteString("\n\n")

	avail := m.height - 5 // header + blank + help
	if avail < 5 {
		avail = 10
	}

	start := m.scopeOffset
	end := start + avail
	if end > len(m.scopeRows) {
		end = len(m.scopeRows)
	}

	for i := start; i < end; i++ {
		row := m.scopeRows[i]
		indent := strings.Repeat("  ", row.Depth)
		isSelected := i == m.scopeCursor

		cursor := "  "
		if isSelected {
			cursor = ftCurStyle.Render("> ")
		}

		switch row.Kind {
		case ScopeRowHost:
			state := m.branchState(row.Children)
			label := ftHostStyle.Render(row.Label)
			if isSelected {
				label = ftSelStyle.Render(row.Label)
			}
			b.WriteString(cursor + indent + state + " " + label + "\n")

		case ScopeRowNamespace:
			state := m.branchState(row.Children)
			label := ftNsStyle.Render(row.Label)
			if isSelected {
				label = ftSelStyle.Render(row.Label)
			}
			b.WriteString(cursor + indent + state + " " + label + "\n")

		case ScopeRowProject:
			var check string
			if m.scopeChecked[row.ProjName] {
				check = ftCheckStyle.Render("[✓]")
			} else {
				check = "[ ]"
			}
			label := row.Label
			if isSelected {
				label = ftSelStyle.Render(label)
			}
			b.WriteString(cursor + indent + check + " " + label + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ftHelpStyle.Render("↑↓/jk: nav  Space/Enter: toggle  Ctrl-Z: undo  Esc: back  q: save & quit"))
	return b.String()
}
```

- [ ] **Step 2: Build**

Run: `go build ./features/`
Expected: compiles

- [ ] **Step 3: Commit**

```bash
git add features/tui.go
git commit -m "Add scope tree rendering with tri-state checkboxes"
```

---

### Task 7: Key handling and screen transitions

**Files:**
- Modify: `features/tui.go`

- [ ] **Step 1: Add Update method**

```go
func (m *featuresModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.screen == screenGateList {
			return m.updateGateList(msg)
		}
		return m.updateScopeTree(msg)
	}
	return m, nil
}

func (m *featuresModel) View() string {
	if m.screen == screenScopeTree {
		return m.viewScopeTree()
	}
	return m.viewGateList()
}

func (m *featuresModel) updateGateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.save()
		return m, tea.Quit
	case tea.KeyUp:
		if m.gateCursor > 0 {
			m.gateCursor--
		}
	case tea.KeyDown:
		if m.gateCursor < len(m.gates)-1 {
			m.gateCursor++
		}
	case tea.KeyEnter:
		if len(m.gates) > 0 {
			m.enterScopeTree()
		}
	case tea.KeySpace:
		if len(m.gates) > 0 {
			m.quickToggleGate()
		}
	default:
		switch msg.String() {
		case "q":
			m.save()
			return m, tea.Quit
		case "k":
			if m.gateCursor > 0 {
				m.gateCursor--
			}
		case "j":
			if m.gateCursor < len(m.gates)-1 {
				m.gateCursor++
			}
		}
	}
	return m, nil
}

func (m *featuresModel) updateScopeTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.applyScopeChanges()
		m.save()
		return m, tea.Quit
	case tea.KeyEsc:
		m.applyScopeChanges()
		m.screen = screenGateList
		return m, nil
	case tea.KeyUp:
		m.scopeMovePrev()
	case tea.KeyDown:
		m.scopeMoveNext()
	case tea.KeySpace, tea.KeyEnter:
		m.scopeToggle()
	default:
		switch msg.String() {
		case "q":
			m.applyScopeChanges()
			m.save()
			return m, tea.Quit
		case "k":
			m.scopeMovePrev()
		case "j":
			m.scopeMoveNext()
		case "ctrl+z":
			m.scopeUndo()
		}
	}
	return m, nil
}
```

- [ ] **Step 2: Add navigation helpers**

```go
func (m *featuresModel) scopeMoveNext() {
	if m.scopeCursor < len(m.scopeRows)-1 {
		m.scopeCursor++
		m.scopeUpdateViewport()
	}
}

func (m *featuresModel) scopeMovePrev() {
	if m.scopeCursor > 0 {
		m.scopeCursor--
		m.scopeUpdateViewport()
	}
}

func (m *featuresModel) scopeUpdateViewport() {
	avail := m.height - 5
	if avail < 5 {
		avail = 10
	}
	if m.scopeCursor >= m.scopeOffset+avail {
		m.scopeOffset = m.scopeCursor - avail + 1
	}
	if m.scopeCursor < m.scopeOffset {
		m.scopeOffset = m.scopeCursor
	}
}
```

- [ ] **Step 3: Add toggle and undo logic**

```go
func (m *featuresModel) scopeToggle() {
	row := m.scopeRows[m.scopeCursor]

	// Build undo diff before changing.
	diff := make(map[string]bool)

	switch row.Kind {
	case ScopeRowProject:
		diff[row.ProjName] = m.scopeChecked[row.ProjName]
		m.scopeChecked[row.ProjName] = !m.scopeChecked[row.ProjName]

	case ScopeRowHost, ScopeRowNamespace:
		// Determine target state: if any unchecked → check all, else uncheck all.
		anyUnchecked := false
		for _, name := range row.Children {
			if !m.scopeChecked[name] {
				anyUnchecked = true
				break
			}
		}
		target := anyUnchecked // true = check all
		for _, name := range row.Children {
			diff[name] = m.scopeChecked[name]
			m.scopeChecked[name] = target
		}
	}

	if len(diff) > 0 {
		m.undoStack = append(m.undoStack, diff)
	}
}

func (m *featuresModel) scopeUndo() {
	if len(m.undoStack) == 0 {
		return
	}
	diff := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	for name, prev := range diff {
		m.scopeChecked[name] = prev
	}
}
```

- [ ] **Step 4: Add screen transition and save helpers**

```go
func (m *featuresModel) enterScopeTree() {
	g := m.gates[m.gateCursor]

	// Collect in-scope projects.
	projects := make(map[string]ScopeProject)
	for projName, proj := range m.cfg.Projects {
		host, path, _, err := config.ParseRemoteURL(proj.Remote)
		if err != nil {
			continue
		}
		if ProjectMatchesScopeByName(projName, host, path, g.Gate.Scope) {
			projects[projName] = ScopeProject{Host: host, Path: path}
		}
	}

	// Hydrate checked state from current feature state.
	fs := m.st.Features[g.Name]
	var checked map[string]bool
	if !fs.Enabled {
		// Gate disabled: nothing checked.
		checked = make(map[string]bool, len(projects))
		for name := range projects {
			checked[name] = false
		}
	} else {
		checked = OverrideToCheckedState(fs.OverrideScope, projects)
	}

	m.scopeGate = g.Name
	m.scopeRows = BuildScopeTree(projects)
	m.scopeChecked = checked
	m.scopeProjects = projects
	m.scopeCursor = 0
	m.scopeOffset = 0
	m.undoStack = nil
	m.screen = screenScopeTree
}

// ProjectMatchesScopeByName checks if a project matches a config scope (not override).
func ProjectMatchesScopeByName(projName, host, path string, scope config.FeatureScope) bool {
	if len(scope.Projects) == 0 && len(scope.GitlabGroups) == 0 && len(scope.GithubOrgs) == 0 {
		return true // empty scope = all projects
	}
	for _, p := range scope.Projects {
		if p == projName {
			return true
		}
	}
	for _, gs := range scope.GitlabGroups {
		if gs.Host == host && (path == gs.Group || strings.HasPrefix(path, gs.Group+"/")) {
			return true
		}
	}
	for _, ghs := range scope.GithubOrgs {
		if ghs.Host == host && strings.HasPrefix(path, ghs.Org+"/") {
			return true
		}
	}
	return false
}

func (m *featuresModel) quickToggleGate() {
	g := m.gates[m.gateCursor]
	fs, ok := m.st.Features[g.Name]
	if ok && fs.Enabled {
		// Disable
		delete(m.st.Features, g.Name)
	} else {
		// Enable with full scope
		m.st.Features[g.Name] = state.FeatureState{Enabled: true}
	}
}

func (m *featuresModel) applyScopeChanges() {
	// Convert checked state to override.
	anyChecked := false
	for _, v := range m.scopeChecked {
		if v {
			anyChecked = true
			break
		}
	}

	if !anyChecked {
		// All unchecked → disable gate.
		delete(m.st.Features, m.scopeGate)
		return
	}

	override := CheckedStateToOverride(m.scopeChecked, m.scopeProjects)
	m.st.Features[m.scopeGate] = state.FeatureState{
		Enabled:       true,
		OverrideScope: override,
	}
}

func (m *featuresModel) save() {
	_ = state.Save(m.cwd, m.st)
}
```

- [ ] **Step 5: Build and verify**

Run: `go build ./features/`
Expected: compiles

- [ ] **Step 6: Commit**

```bash
git add features/tui.go
git commit -m "Add key handling, screen transitions, toggle, and undo"
```

---

### Task 8: Run entry point and command wiring

**Files:**
- Modify: `features/tui.go`
- Modify: `cmd/features.go`

- [ ] **Step 1: Add Run function to features/tui.go**

```go
// Run starts the feature gate TUI.
func Run(cfg *config.GitteConfig, cwd string, st *state.GitteState) error {
	if len(cfg.FeatureGates) == 0 {
		fmt.Println("No feature gates defined in configuration.")
		return nil
	}
	model := newFeaturesModel(cfg, cwd, st)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

- [ ] **Step 2: Update cmd/features.go to launch TUI**

Replace the `newFeaturesCmd` function to add a `RunE` that launches the TUI when no subcommand is given:

```go
func newFeaturesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "features",
		Short: "Manage feature gates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputMode() == output.ModePlain {
				// Non-TTY: fall back to list
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
```

Add imports: `"github.com/cego/gitte/features"` and `"github.com/cego/gitte/output"`.

- [ ] **Step 3: Extend enable command with new flags**

Update `newFeaturesEnableCmd` in `cmd/features.go`:

```go
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

			// Build override scope if any flags specified.
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
```

- [ ] **Step 4: Build and test full binary**

Run: `go build ./... && go test ./...`
Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add features/tui.go cmd/features.go
git commit -m "Wire features TUI to command and extend enable flags"
```

---

## Chunk 3: Integration and Polish

### Task 9: Manual smoke test

- [ ] **Step 1: Run the TUI**

Run: `go run . features` (from the gitte repo root, with a config that has feature gates)

Verify:
- Gate list shows with correct status indicators
- Space toggles enabled/disabled
- Enter drills into scope tree
- Scope tree shows projects grouped by host/namespace
- Space/Enter toggles nodes (branches toggle all children)
- Tri-state checkboxes update correctly
- Ctrl-Z undoes the last toggle
- Esc goes back to gate list
- q saves and quits
- Check `.gitte-state.yml` for correct output

- [ ] **Step 2: Test non-TTY mode**

Run: `go run . features --no-tty`
Expected: prints gate list (text output)

Run: `go run . features enable dev-build --gitlab-group gitlab.cego.dk/cego --exclude mysql`
Expected: saves state with override

- [ ] **Step 3: Fix any issues found**

---

### Task 10: Final build and test

- [ ] **Step 1: Run full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all pass

- [ ] **Step 2: Final commit if any fixes were made**

```bash
git add -A
git commit -m "Polish features TUI and fix issues from smoke testing"
```
