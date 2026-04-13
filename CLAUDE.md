# CLAUDE.md

This file documents conventions and guidelines for working in this repository.

## Project overview

Gitte is a Go CLI tool for developer environment orchestration — it manages git repos, runs startup checks, and executes ordered actions across projects. Entry point is `main.go`; all commands are in `cmd/`.

## Build & test

```bash
go build ./...          # compile
go test ./...           # run all tests
go vet ./...            # static analysis
```

There is no Makefile. Use the standard Go toolchain directly.

## Code style

- Follow standard Go conventions: `gofmt`, `go vet`, standard naming.
- Exported symbols get doc comments. Unexported code only gets comments where the logic is non-obvious.
- Error messages are lowercase and don't end with punctuation (Go convention).
- Wrap errors with `fmt.Errorf("context: %w", err)`, not `errors.New`.
- Return errors; don't `log.Fatal` or `os.Exit` outside of `main`.

## Architecture

```
cmd/        cobra commands — thin wrappers that call packages, no business logic
config/     config loading, type definitions, template resolution, startup checks
executor/   task runner — dependency resolution, parallelism, retry
actions/    action planner and runner
gitops/     git clone/pull, discovery from GitLab/GitHub APIs
startup/    startup check orchestration and view (plain + TUI)
output/     TTY detection, plain writer, BubbleTea run TUI
state/      .gitte-state.yml load/save (toggles, feature gates, cache)
toggle/     BubbleTea toggle TUI
```

### Key patterns

**Executor hooks** — the executor calls `OnTaskStart(name)` and `OnTaskFinish(name, err, elapsed)` callbacks. Wire these to whatever output system you're using (plain writer or TUI).

**Output mode** — `output.IsTTY()` drives which view is used. Respect `--no-tty` flag and `GITTE_NO_TTY=1`. Never write directly to stdout from business logic packages; go through the view/output layer.

**Context threading** — `config.ConfigFromContext` and `config.CwdFromContext` retrieve globals from context. Don't add new context values; pass explicit parameters instead.

**BubbleTea programs** — use `tea.NewProgram(model)` without `tea.WithAltScreen()` for inline rendering. Quit via channel close + `allDoneMsg` pattern (see `startup/view.go`). Print the final model view after `p.Run()` returns since BubbleTea clears its inline output on exit.

## Adding a new command

1. Create `cmd/<name>.go` with a `newXxxCmd()` function returning `*cobra.Command`.
2. Register it in `cmd/root.go` under `rootCmd.AddCommand(...)`.
3. Keep the command's `RunE` thin — validate args, call into a package, return the error.

## Adding a new startup check type

1. Implement the `StartupCheck` interface in `config/startup_checks.go`.
2. Add a new `case` in `StartupCheckMap.UnmarshalYAML` for the new `type` string.

## Testing

- Unit tests live alongside source files (`_test.go`).
- Tests use the standard `testing` package only — no third-party assertion libraries.
- Table-driven tests are preferred for multiple input cases.
- Test function names: `TestPackageName_ScenarioDescription`.
- Don't test private implementation details; test observable behaviour.
- `go test ./...` must pass before committing.

## Dependencies

- Use the standard library where possible.
- Adding a new dependency requires a clear justification.
- Run `go mod tidy` after adding or removing imports.
- Key dependencies: `cobra` (CLI), `bubbletea` + `lipgloss` (TUI), `goccy/go-yaml` (YAML + JSONPath), `golang.org/x/term` (TTY detection).

## Git

- Branch from `main`. Use descriptive branch names.
- Never commit directly to `main`.
- Never force-push to `main`.
- Do not push without explicit instruction.
