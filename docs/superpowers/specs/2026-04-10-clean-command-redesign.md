---
date: 2026-04-10
topic: clean command redesign
status: approved
---

# gitte clean — Subcommand Redesign

## Background

The 1.x `gitte clean` command performed destructive cleanup operations (untracked removal, hard reset, branch checkout) across all repos. The 2.x rewrite replaced it with flag-based reporting only, which diverges significantly from 1.x behavior. This spec restores the destructive operations as subcommands, with improved UX for the interactive `local-changes` flow, and drops the `--non-gitte` operation entirely.

## Command Structure

```
gitte clean                     # shows cobra help, does nothing
gitte clean all                 # runs: untracked → local-changes → master
gitte clean untracked           # git clean -fdx in every repo (no prompt)
gitte clean local-changes       # interactive: list repos, prompt mode, reset
gitte clean master              # git checkout <default_branch> in every repo (no prompt)
```

The root `clean` command has no `RunE` — cobra's default help is shown when invoked without a subcommand.

## Subcommand Behaviour

### untracked

Iterates all configured projects (regardless of toggle state) that exist on disk. Runs `git clean -fdx` in each. Errors are printed as warnings and iteration continues. No confirmation prompt.

### local-changes

1. Run `git status --porcelain` across all repos; collect those with any output.
2. If none: print "No repos with local changes." and exit 0.
3. Print the list of affected repos.
4. Prompt: `Reset all, handle individually, or cancel? [all/individually/cancel]:`
5. `all` — run `git reset --hard` in each affected repo.
6. `individually` — for each repo prompt `Reset <name>? [y/N]:`, skip on N or empty input.
7. `cancel` (or empty input, or unrecognised input) — exit 0 with no changes.

Errors per repo are printed as warnings; iteration continues.

### master

Iterates all configured projects (regardless of toggle state) that exist on disk. Runs `git checkout <default_branch>` in each (using `proj.DefaultBranch`, falling back to `"master"`). Errors are printed as warnings. No confirmation prompt.

### all

Calls the three subcommand functions in order: untracked → local-changes → master.

## Error Handling

- Per-repo errors (non-zero exit, I/O failure) are printed to stderr as `warning: [name] <message>` and do not stop the run.
- The command exits non-zero only on fatal startup errors (config not found, etc.).

## Removed

- `--untracked`, `--local-changes`, `--master`, `--non-gitte` flags
- `checkNonGitte` function and all non-gitte directory scanning logic
- Reporting-only behaviour (the old `checkLocalState`, `checkBranch` functions)

## File Changes

Single file: `cmd/clean.go`. No other files are affected.

## Out of Scope

- `--non-gitte` cleanup (intentionally excluded)
- Running `gitte gitops` sync after clean (1.x did this; 2.x has a separate `gitte gitops` command)
- TUI/BubbleTea interactive selection
