# Feature Gate TUI Design

## Overview

Interactive BubbleTea TUI for toggling feature gates and customizing their scope per machine. Gate-first navigation: a gate list screen leads into a scope tree screen where the user checks/unchecks branches and projects. Changes persist to `.gitte-state.yml`.

## State Model

### Extended ScopeOverride (`state/state.go`)

```go
type ScopeOverride struct {
    Projects     []string              `yaml:"projects,omitempty"`
    GitlabGroups []ScopeOverrideGroup  `yaml:"gitlab_groups,omitempty"`
    GithubOrgs   []ScopeOverrideOrg    `yaml:"github_orgs,omitempty"`
}

type ScopeOverrideGroup struct {
    Host            string   `yaml:"host"`
    Group           string   `yaml:"group"`
    ExcludeProjects []string `yaml:"exclude_projects,omitempty"`
}

type ScopeOverrideOrg struct {
    Host            string   `yaml:"host"`
    Org             string   `yaml:"org"`
    ExcludeProjects []string `yaml:"exclude_projects,omitempty"`
}
```

The existing `Projects []string` field is preserved for backward compatibility. New fields are additive.

`Group` uses prefix matching with a `/` delimiter — `group: spilnu` matches `spilnu/services/promotion` but not `spilnu-legacy/something`. This maps naturally to tree namespace levels. (Note: this fixes a pre-existing inconsistency where GitLab group matching lacked the `/` delimiter that GitHub org matching already uses.)

### Scope Resolution

In `actions/runner.go`, a new `projectMatchesOverrideScope` function resolves the override scope directly (not by converting to `config.FeatureScope`, since `FeatureScope` has no exclude fields):

1. If `OverrideScope` is nil → use the gate's config scope (full scope).
2. If `OverrideScope` is set:
   - Check `Projects` list → included. (`Projects` is used for gates whose config scope includes explicit project names not belonging to any group/org.)
   - Check `GitlabGroups`: if project's host matches and path has prefix `group + "/"` (or equals `group`), AND project config key is NOT in that entry's `ExcludeProjects` → included.
   - Check `GithubOrgs`: same logic with `org + "/"` prefix matching.
   - Otherwise → excluded.

If the resulting override is empty (no projects, no groups, no orgs), the gate is disabled (`enabled: false`).

## Screen 1: Gate List

Flat list of all feature gates from the config, sorted alphabetically by gate name. Each row:

```
[✓] dev-build — Build from source instead of using latest production tag
[ ] debug-logging — Enable verbose debug logging for all services
[·] experimental-api — Use new API v2 endpoints
```

### Status Indicators

- `[✓]` enabled, full config scope (no override or override matches full scope)
- `[·]` enabled, narrowed scope (override excludes some projects)
- `[ ]` disabled

### Navigation

| Key | Action |
|-----|--------|
| j / k / arrows | Move cursor |
| Space | Quick toggle: enabled → disabled, disabled → enabled (full scope) |
| Enter | Drill into scope tree for selected gate |
| q / Esc | Save and quit |
| Ctrl-C | Save and quit |

When no feature gates are defined, show a message and exit.

### Command Entry Point

`gitte features` launches the TUI when TTY is detected. Falls back to `gitte features list` behavior in non-TTY mode. Existing `list`/`enable`/`disable` subcommands remain for scripting.

## Screen 2: Scope Tree

Shown after pressing Enter on a gate. Displays projects within the gate's configured scope, grouped by host → namespace segments → project (same tree pattern as actions TUI and toggle TUI). The tree uses the full namespace depth from `ParseRemoteURL` — e.g., `gitlab.cego.dk / cego / services / promotion` has host `gitlab.cego.dk`, namespace segments `cego` and `services`, and leaf `promotion`.

If a gate's configured scope is empty (applies to all projects), the scope tree shows all projects from the config.

```
← dev-build — Build from source instead of...

[·] gitlab.cego.dk
  [✓] cego
      [✓] monolith
      [✓] mysql
      [✓] laravel-bundle
      [ ] some-excluded-project
  [ ] spilnu
      [ ] promotion
      [ ] user-auth
```

### Header

Gate name + truncated description. `←` indicates Esc goes back.

### Tri-State Checkboxes

- `[✓]` all children checked
- `[·]` some children checked (partial)
- `[ ]` no children checked

Branch state is derived from children, not stored independently.

### Navigation

| Key | Action |
|-----|--------|
| j / k / arrows | Move cursor (all nodes selectable — branches and leaves) |
| Space / Enter | Toggle node under cursor |
| Ctrl-Z | Undo last toggle |
| Esc | Back to gate list (changes held in memory until q) |
| q | Save and quit |
| Ctrl-C | Save and quit |

### Toggle Behavior

- Toggle `[ ]` or `[·]` node → check it and all descendants.
- Toggle `[✓]` node → uncheck it and all descendants.

### Undo (Ctrl-Z)

Each toggle pushes the previous checked state of affected nodes onto an undo stack (stores only the diff: affected project names and their prior boolean state). Ctrl-Z pops and restores. Stack resets when leaving the tree.

### Scope Persistence

On save, the checked state converts to `ScopeOverride`:

- All nodes checked → `override_scope` is nil (full config scope applies).
- No nodes checked → gate disabled (`enabled: false`).
- Otherwise, build override from checked state:
  - Fully-checked namespace level → `gitlab_groups` entry (group = full path prefix up to that level, e.g., `cego` or `cego/services`) or `github_orgs` entry.
  - Partially-checked namespace level → same entry with `exclude_projects` listing unchecked leaf project config keys under that namespace.
  - Projects from the config scope's explicit `Projects` list that are checked → added to override `Projects` list.

Serialization rule: the algorithm walks the tree bottom-up. At each namespace level, if all children are checked, emit a single group/org entry. If some are unchecked, emit the group/org entry with `exclude_projects`. Individual projects not covered by any group/org entry go into the `Projects` list.

## Non-TTY CLI

Extended subcommands for scripting:

```
gitte features list                                      # list gates with status
gitte features enable <gate>                             # enable for full config scope
gitte features enable <gate> --project <name>            # override to specific project(s)
gitte features enable <gate> --gitlab-group host/group   # override to gitlab group
gitte features enable <gate> --github-org host/org       # override to github org
gitte features enable <gate> --exclude <project>         # exclude from all matching groups/orgs
gitte features disable <gate>                            # disable entirely
```

Multiple `--project`, `--gitlab-group`, `--github-org`, and `--exclude` flags can be combined. If no scope flags specified, full config scope applies.

`--exclude` applies globally: the excluded project config key is added to the `ExcludeProjects` list of every group/org entry where that project would otherwise match.

`gitte features list` shows effective scope per gate.

## File Structure

### New Package: `features/`

- `features/tui.go` — BubbleTea model, gate list rendering, scope tree rendering, key handling.
- `features/scope.go` — scope tree building (projects within a gate's config scope grouped into tree rows), checked-state-to-ScopeOverride conversion, ScopeOverride-to-checked-state hydration, `projectMatchesOverrideScope` function.

### Modified Files

- `state/state.go` — extend `ScopeOverride` with `GitlabGroups`, `GithubOrgs`, sub-types.
- `actions/runner.go` — update scope resolution to use `projectMatchesOverrideScope` from `features/scope.go` (or extract to a shared location) for handling extended `ScopeOverride` with exclude lists. Fix GitLab group prefix matching to use `/` delimiter.
- `cmd/features.go` — launch TUI when TTY detected; extend `enable` subcommand with `--gitlab-group`, `--github-org`, `--exclude` flags.

## Technical Decisions

- **Single BubbleTea model** with a `screen` enum (`screenGateList` / `screenScopeTree`). Enter transitions forward, Esc transitions back. One `tea.Program`, no nesting.
- **Alt-screen rendering** (`tea.WithAltScreen()`), matching the toggle TUI which is the closest analogue. The two-screen navigation and potentially large trees benefit from full-screen display.
- **Save on quit**: q, Esc (from gate list), and Ctrl-C all save to `.gitte-state.yml`. Esc from scope tree goes back to gate list without saving yet (changes held in memory until the user leaves the gate list).
- **Tree building reuses existing patterns**: `config.ParseRemoteURL` for host/path extraction, same host → namespace → leaf grouping as toggle TUI and actions TUI.
- **Styling**: magenta (170) for cursor/selection, cyan (39) for hosts, dim (244) for namespaces, green (42) for checked, consistent with existing TUIs.
