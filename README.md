# gitte

[![license](https://img.shields.io/github/license/cego/gitte)](https://github.com/cego/gitte)

Gitte is a developer environment orchestration tool for teams working across many git repositories. It keeps all your repos in sync, runs startup checks to verify your local machine is ready, and executes ordered actions (build, up, down, etc.) across projects with dependency resolution.

## Features

- **Startup checks** — verify tools, versions, and credentials before doing anything
- **Git sync** — clone and pull all configured repositories in parallel
- **Ordered actions** — run commands across projects with `needs`-based dependency ordering and parallel execution
- **Project toggles** — interactively enable/disable projects per machine
- **Template inheritance** — share action definitions across similar projects
- **Feature gates** — toggle environment-variable-based features per machine
- **Auto-discovery** — discover and sync repositories from GitLab groups or GitHub orgs
- **TTY-aware output** — spinner TUI in a terminal, structured plain-text output in CI/non-TTY

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap cego/gitte https://github.com/cego/gitte
brew install cego/gitte/gitte
```

### APT (Debian and Ubuntu)

```bash
curl -fsSL https://gitte-ppa.cego.dk/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/gitte.gpg
echo "deb [signed-by=/etc/apt/keyrings/gitte.gpg] https://gitte-ppa.cego.dk ./" | sudo tee /etc/apt/sources.list.d/gitte.list
sudo apt update
sudo apt install gitte
```

### Binary download

Download the latest binary from the [releases page](https://github.com/cego/gitte/releases) and place it on your `PATH`.

### Go install

Requires Go 1.24 or later.

```bash
go install github.com/cego/gitte@latest
```

## Quick start

Create a `.gitte.yml` in the root of your workspace:

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
          prod: ["docker", "compose", "up", "-d"]
      down:
        groups:
          prod: ["docker", "compose", "down"]
```

Then run:

```bash
gitte run up prod
```

Gitte will run startup checks, pull all repos, then execute the `up` action for group `prod` on all enabled projects.

## Commands

| Command | Description |
|---------|-------------|
| `gitte run [action] [group] [projects]` | Full pipeline: startup checks → git sync → actions |
| `gitte actions [action] [group] [projects]` | Run actions only (skip startup and git sync) |
| `gitte startup` | Run startup checks only |
| `gitte gitops [--discover]` | Clone/pull all repos; `--discover` also fetches from configured sources |
| `gitte toggle` | Interactive TUI to enable/disable projects |
| `gitte features list` | List all feature gates and their enabled state |
| `gitte features enable <gate>` | Enable a feature gate |
| `gitte features disable <gate>` | Disable a feature gate |
| `gitte validate` | Validate config: schema, cycles, missing references |
| `gitte clean [flags]` | Cleanup repos (see below) |

### Argument syntax

Arguments to `run` and `actions` are positional: `action [group] [projects]`.

```bash
gitte run up                      # up on all enabled projects, all groups
gitte run up myservice            # up on myservice only, all groups
gitte run up myservice prod       # up on myservice, group prod only
gitte run up frontend+backend prod  # up on frontend and backend, group prod
gitte run up * prod               # up on all enabled projects, group prod
gitte run up+build                # run up then build on all projects
```

Use `*` or `all` as a wildcard. Combine multiple values with `+`.

### clean flags

```bash
gitte clean --untracked        # git clean -fdx in all repos
gitte clean --local-changes    # discard uncommitted changes (prompts for confirmation)
gitte clean --master           # checkout default branch in all repos
gitte clean --non-gitte        # remove directories not managed by gitte (prompts)
```

## Configuration

See [docs/config.md](./docs/config.md) for the full configuration reference.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GITTE_CWD` | process cwd | Override the working directory |
| `GITTE_NO_TTY` | `0` | Set to `1` to force plain-text output (no TUI) |
| `GITTE_NO_NEEDS` | `0` | Set to `1` to ignore `needs` dependencies |
| `GITTE_MAX_TASK_PARALLELIZATION` | unlimited | Cap the number of concurrent tasks |

## Global flags

```
--config <path>    Path to .gitte.yml (default: auto-detected by walking up)
--cwd <path>       Override working directory
--no-tty           Force plain-text output
```

## State file

Gitte stores per-machine state (project toggles, feature gate overrides, remote config cache) in `.gitte-state.yml` next to your `.gitte.yml`. This file is automatically added to `.gitignore`.

## Override file

If `.gitte-override.yml` exists alongside `.gitte.yml`, it is deep-merged on top. Use this for local machine overrides that should not be committed.
