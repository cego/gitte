# gitte

[![license](https://img.shields.io/github/license/cego/gitte)](https://github.com/cego/gitte)

**Monorepo-like development across many git repositories — without the monorepo.**

Gitte keeps all your repos in sync, runs startup checks to verify your local machine is ready, and executes ordered actions (build, up, down, …) across projects with dependency resolution and parallel execution.

---

## Table of contents

- [Features](#features)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Commands](#commands)
- [Configuration](#configuration)
- [Environment variables](#environment-variables)
- [Global flags](#global-flags)
- [State and override files](#state-and-override-files)

---

## Features

- **Startup checks** — verify tools, versions, and credentials before doing anything
- **Git sync** — clone and pull all configured repositories in parallel
- **Ordered actions** — run commands across projects with `needs`-based dependency ordering and parallel execution
- **Project toggles** — interactively enable/disable projects per machine
- **Template inheritance** — share action definitions across similar projects
- **Feature gates** — toggle environment-variable-based behaviours per machine
- **Auto-discovery** — discover and sync repositories from GitLab groups or GitHub orgs
- **Clean operations** — remove untracked files, reset local changes, or check out default branches across all repos
- **TTY-aware output** — animated spinner TUI in a terminal, structured plain-text output in CI / non-TTY

---

## Installation

### APT (Debian and Ubuntu)

```bash
curl -fsSL https://gitte-apt.cego.dk/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/gitte.gpg
echo "deb [signed-by=/etc/apt/keyrings/gitte.gpg] https://gitte-apt.cego.dk ./" | sudo tee /etc/apt/sources.list.d/gitte.list
sudo apt update
sudo apt install gitte
```

### Homebrew (macOS and Linux)

```bash
brew tap cego/gitte https://github.com/cego/gitte
brew install cego/gitte/gitte
```

### Binary download

Download the latest binary from the [releases page](https://github.com/cego/gitte/releases) and place it on your `PATH`.

### Go install

Requires Go 1.24 or later.

```bash
go install github.com/cego/gitte@latest
```

---

## Quick start

**1.** Create a `.gitte.yml` in your workspace root:

```yaml
startup:
  git-present:
    type: command
    cmd: ["git", "--version"]
    hint: "git is not installed"

projects:
  myservice:
    remote: git@github.com:example/myservice.git
    default_branch: main
    actions:
      up:
        groups:
          local: ["docker", "compose", "up", "-d"]
      down:
        groups:
          local: ["docker", "compose", "down"]
```

**2.** Run the full pipeline:

```bash
gitte run up local
```

Gitte will run startup checks, pull all repos, then execute the `up` action for group `local` across all enabled projects.

---

## Commands

### Overview

| Command | Description |
|---------|-------------|
| `gitte run [action] [group] [projects]` | Full pipeline: startup checks → git sync → actions |
| `gitte actions [action] [group] [projects]` | Run actions only (skip startup and git sync) |
| `gitte startup` | Run startup checks only |
| `gitte gitops [--discover]` | Clone/pull all repos; `--discover` also fetches from configured sources |
| `gitte list` | List all projects and their available actions |
| `gitte toggle` | Interactive TUI to enable/disable projects per machine |
| `gitte clean <subcommand>` | Clean up repos (see below) |
| `gitte features list\|enable\|disable` | Manage feature gates |
| `gitte sources` | Manage discovery sources (GitLab groups / GitHub orgs) |
| `gitte token set\|get\|delete\|list` | Store API tokens in the system keyring |
| `gitte validate` | Validate config: schema, cycles, missing references |

### Argument syntax

Arguments to `run` and `actions` are positional: `action [group] [projects]`.

```bash
gitte run up                         # up action, all groups, all enabled projects
gitte run up local                   # up action, group local, all enabled projects
gitte run up local myservice         # up action, group local, project myservice only
gitte run up local frontend+backend  # up action, group local, projects frontend and backend
gitte run up+build                   # run up then build, all groups, all enabled projects
```

Use `*` or `all` as a wildcard. Combine multiple values with `+`.

### gitte clean

Destructive cleanup operations across all configured repositories. Each subcommand shows a live progress TUI while running.

```bash
gitte clean untracked       # remove untracked files (git clean -fdx)
gitte clean local-changes   # reset repos with local changes (prompts before acting)
gitte clean master          # checkout the default branch in all repos
gitte clean all             # run all three in sequence
```

`gitte clean local-changes` lists affected repos and prompts:

```
Reset [a]ll / [i]ndividually / [C]ancel?
```

---

## Configuration

See [docs/config.md](./docs/config.md) for the full configuration reference.

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GITTE_CWD` | process cwd | Override the working directory |
| `GITTE_NO_TTY` | `0` | Set to `1` to force plain-text output (no TUI) |
| `GITTE_NO_NEEDS` | `0` | Set to `1` to ignore `needs` dependencies |
| `GITTE_NO_REBASE` | `false` | Set to `true` to skip auto-rebase onto default branch |
| `GITTE_MAX_TASK_PARALLELIZATION` | unlimited | Cap the number of concurrent tasks |

---

## Global flags

```
--config <path>    Path to .gitte.yml (default: auto-detected by walking up)
--cwd <path>       Override working directory
--no-tty           Force plain-text output
```

---

## State and override files

**`.gitte-state.yml`** — stores per-machine state: project toggles, feature gate overrides, and remote config cache. Gitte automatically adds this file to `.gitignore`. Do not commit it.

**`.gitte-override.yml`** — deep-merged on top of `.gitte.yml` when present. Use this for local machine-specific settings that should not be committed (e.g. `sources` added via `gitte sources add`).
