# Commands

All commands support the global flags `--config <path>`, `--cwd <path>`, and `--no-tty`.

---

## gitte run

Full pipeline: startup checks → git sync → actions.

```bash
gitte run up
gitte run up prod
gitte run up myservice prod
gitte run up frontend+backend prod
gitte run up * prod
gitte run up+build
gitte run up --discover    # also fetch repos from configured sources first
```

Arguments:
1. **action** — action name(s), `+`-separated. Required.
2. **projects** — project name(s) or `*` for all. Optional, defaults to all enabled.
3. **group** — group name or `*` for all. Optional, defaults to all.

---

## gitte actions

Run actions only, skipping startup checks and git sync. Same argument syntax as `run`.

```bash
gitte actions up
gitte actions up prod
gitte actions down myservice prod
```

---

## gitte startup

Run startup checks only.

```bash
gitte startup
```

Exits non-zero if any check fails. In TTY mode shows a live progress list; in non-TTY mode prints structured lines:

```
[startup:git-present] RUNNING
[startup:git-present] OK (12ms)
[startup:docker-version] RUNNING
[startup:docker-version] FAILED (45ms): shell script exited with code 1
hint: Docker must be at least version 25.0.0
```

---

## gitte gitops

Clone or pull all configured projects.

```bash
gitte gitops
gitte gitops --discover    # also fetch repos from configured sources
```

- Repos are cloned into `<workspace>/<host>/<namespace>/<repo>`.
- If a repo has local changes, it is skipped (with a warning).
- Only fast-forward pulls are performed.

---

## gitte toggle

Open an interactive TUI to enable or disable projects on this machine. Projects marked `defaultDisabled: true` in the config are off by default and must be explicitly enabled here.

```bash
gitte toggle
```

State is saved to `.gitte-state.yml`.

---

## gitte features

Manage feature gates — opt-in behaviours that inject environment variables into matching action runs.

```bash
gitte features list                 # show all gates and their state
gitte features enable HOT_RELOAD    # enable a gate
gitte features disable HOT_RELOAD   # disable a gate
```

---

## gitte validate

Parse and validate the configuration file. Reports schema errors, unknown template references, unknown `needs` targets, and dependency cycles. Exits non-zero if any errors are found.

```bash
gitte validate
```

---

## gitte clean

Clean up project repositories.

```bash
gitte clean --untracked       # remove untracked files (git clean -fdx)
gitte clean --local-changes   # discard uncommitted changes (prompts for confirmation)
gitte clean --master          # checkout the default branch in all repos
gitte clean --non-gitte       # remove directories not managed by gitte (prompts)
```

Multiple flags can be combined. If no flags are given, all operations run.
