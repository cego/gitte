---
date: 2026-04-10
topic: gitte clean TUI
status: approved
---

# gitte clean TUI Design

## Background

`gitte clean` currently runs operations silently (only printing warnings to stderr). The goal is to add a BubbleTea TUI that shows which cleaning phase is active and the per-repo progress, consistent with how gitops and startup already render their work.

## Layout

Phase sections rendered sequentially. Each section shows a header line followed by repos in a responsive grid:

- Terminal width ≥ 180 cols → 3 columns
- Terminal width ≥ 120 cols → 2 columns
- Otherwise → 1 column

Upcoming phases are shown as dimmed headers below the active section. Completed phases remain visible above.

Each repo row:

```
<icon> <host>/<namespace>/<repo>   <detail>
```

Icons: `○` pending, braille spinner (10-frame) running, `✓` ok, `✗` failed.

Example (wide terminal, `gitte clean all` mid-run):

```
── Untracked ────────────────────────────────────────────────────────
✓ github.com/acme/api        removed 2 files   ✓ github.com/acme/ui    nothing
⠸ github.com/acme/worker     cleaning…         ○ github.com/acme/docs

── Local Changes ────────────────────────────────────────────────────
── Master ───────────────────────────────────────────────────────────
```

## Subcommand Behaviour

### `clean untracked`

Single-phase TUI (Untracked section only). Starts the BubbleTea program, iterates repos concurrently feeding updates via channel, exits when all repos are done. No post-TUI summary.

### `clean local-changes`

Two-step flow:
1. **Scan phase** — single-phase TUI showing each repo being checked for local changes. Exits when scan is complete.
2. **Interactive prompt** — runs in plain text after TUI exits, using the existing `bufio.Scanner` prompt flow. Same pattern gitops uses for post-TUI checkout prompts.

### `clean master`

Single-phase TUI (Master section only). Same structure as `clean untracked`.

### `clean all`

All three phases in one TUI program. Phases run sequentially: untracked completes → local-changes scan completes (TUI pauses, prompt runs, TUI resumes or a new phase starts) → master completes.

For `clean all`, the local-changes interactive prompt runs between phases: TUI pauses after the scan phase, the prompt runs in plain text, then the master phase TUI continues.

## Non-TTY Fallback

When `outputMode() == output.ModePlain`, skip the TUI entirely. Print plain structured lines to stdout:

```
[clean:untracked] acme/api: removed 2 files
[clean:untracked] acme/worker: nothing to clean
[clean:local-changes] acme/api: has local changes
...
```

## Architecture

### New file: `cmd/clean_view.go`

BubbleTea model for the clean TUI. Follows the gitops/view.go pattern:

- `msgCh chan cleanMsg` — buffered channel (size 100) for updates from worker goroutines
- `drainedCh chan struct{}` — signals that the final message has been processed (for clean shutdown)
- `phases []cleanPhase` — ordered list of phases; each phase has title, a slice of `repoEntry`, and a done flag
- `activePhase int` — index of currently running phase
- `spinnerTick int` — incremented on each tick for braille animation
- `width`, `height` — updated on `tea.WindowSizeMsg`

Exposed interface (used by `cmd/clean.go`):

```go
type cleanView struct { ... }

func newCleanView(mode output.OutputMode, phases []string) *cleanView
func (v *cleanView) OnStart(phase, repo string)
func (v *cleanView) SetDetail(phase, repo, detail string)
func (v *cleanView) OnFinish(phase, repo string, err error)
func (v *cleanView) AdvancePhase()   // called between phases in clean all
func (v *cleanView) Wait()           // drain + quit + wait for program exit
```

### Modified: `cmd/clean.go`

Each `runClean*` function:
1. Checks `outputMode()` — if plain, uses existing `fmt.Printf`/`fmt.Fprintf` paths
2. If TTY, creates a `cleanView`, starts BubbleTea, sends updates via view callbacks, calls `view.Wait()` at end

The helper functions (`sortedProjectNames`, `resolveProjectPath`, `hardReset`) are unchanged.

## Out of Scope

- Final summary after TUI exits (gitops prints one; clean does not — the per-repo detail lines are sufficient)
- Keyboard navigation or log focus (not needed for a short-lived cleanup operation)
- Cancellation via Ctrl-C (handled by existing context cancellation)
