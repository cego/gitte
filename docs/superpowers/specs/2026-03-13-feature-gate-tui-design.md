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

`Group` uses the same prefix matching as `config.GitlabScope` — `group: spilnu` matches `spilnu/services/promotion`. This maps naturally to tree namespace levels.

### Scope Resolution

In `actions/runner.go`, when `OverrideScope` is set, construct a `config.FeatureScope` from it and apply exclude lists:

1. If `OverrideScope` is nil → use the gate's config scope (full scope).
2. If `OverrideScope` is set:
   - Check `Projects` list → included.
   - Check `GitlabGroups`: if project's host and path prefix match a group entry AND project name is NOT in that entry's `ExcludeProjects` → included.
   - Check `GithubOrgs`: same logic with org prefix matching.
   - Otherwise → excluded.

If the resulting override is empty (no projects, no groups, no orgs), the gate is disabled (`enabled: false`).

## Screen 1: Gate List

Flat list of all feature gates from the config. Each row:

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
| Enter | Drill into scope tree for selected gate |
| q | Save and quit |

When no feature gates are defined, show a message and exit.

### Command Entry Point

`gitte features` launches the TUI when TTY is detected. Falls back to `gitte features list` behavior in non-TTY mode. Existing `list`/`enable`/`disable` subcommands remain for scripting.

## Screen 2: Scope Tree

Shown after pressing Enter on a gate. Displays projects within the gate's configured scope, grouped by host → namespace → project (same tree pattern as actions TUI and toggle TUI).

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
| Esc | Back to gate list |
| q | Save and quit |

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
  - Fully-checked namespace level → `gitlab_groups` entry (group = path prefix) or `github_orgs` entry.
  - Partially-checked namespace level → same entry with `exclude_projects` listing unchecked project names.
  - Individually checked projects not under any checked namespace → `projects` list.

## Non-TTY CLI

Extended subcommands for scripting:

```
gitte features list                                      # list gates with status
gitte features enable <gate>                             # enable for full config scope
gitte features enable <gate> --project <name>            # override to specific project(s)
gitte features enable <gate> --gitlab-group host/group   # override to gitlab group
gitte features enable <gate> --github-org host/org       # override to github org
gitte features enable <gate> --exclude <project>         # exclude project (combinable)
gitte features disable <gate>                            # disable entirely
```

Multiple `--project`, `--gitlab-group`, `--github-org`, and `--exclude` flags can be combined. If no scope flags specified, full config scope applies.

`gitte features list` shows effective scope per gate.

## File Structure

### New Package: `features/`

- `features/tui.go` — BubbleTea model, gate list rendering, scope tree rendering, key handling.
- `features/scope.go` — scope tree building (projects within a gate's config scope grouped into tree rows), checked-state-to-ScopeOverride conversion, ScopeOverride-to-checked-state hydration.

### Modified Files

- `state/state.go` — extend `ScopeOverride` with `GitlabGroups`, `GithubOrgs`, sub-types.
- `actions/runner.go` — update scope resolution to handle extended `ScopeOverride` with exclude lists.
- `cmd/features.go` — launch TUI when TTY detected; extend `enable` subcommand with `--gitlab-group`, `--github-org`, `--exclude` flags.

## Technical Decisions

- **Single BubbleTea model** with a `screen` enum (`screenGateList` / `screenScopeTree`). Enter transitions forward, Esc transitions back. One `tea.Program`, no nesting.
- **Inline rendering** (no alt-screen), consistent with existing TUIs. Print final state after `p.Run()` returns.
- **Save on quit only**: q from either screen saves to `.gitte-state.yml`. Esc from scope tree goes back to gate list without saving (changes held in memory).
- **Tree building reuses existing patterns**: `config.ParseRemoteURL` for host/path extraction, same host → namespace → leaf grouping as toggle TUI and actions TUI.
- **Styling**: magenta (170) for cursor/selection, cyan (39) for hosts, dim (244) for namespaces, green (42) for checked, consistent with existing TUIs.
