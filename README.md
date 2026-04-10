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
curl -fsSL https://gitte-apt.cego.dk/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/gitte.gpg
echo "deb [signed-by=/etc/apt/keyrings/gitte.gpg] https://gitte-apt.cego.dk ./" | sudo tee /etc/apt/sources.list.d/gitte.list
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
          local: ["docker", "compose", "up", "-d"]
      down:
        groups:
          local: ["docker", "compose", "down"]
```

Then run:

```bash
gitte run up local
```

Gitte will run startup checks, pull all repos, then execute the `up` action for group `local` on all enabled projects.

## Commands

| Command | Description |
|---------|-------------|
| `gitte run [action] [group] [projects]` | Full pipeline: startup checks → git sync → actions |
| `gitte actions [action] [group] [projects]` | Run actions only (skip startup and git sync) |
| `gitte startup` | Run startup checks only |
| `gitte gitops [--discover]` | Clone/pull all repos; `--discover` also fetches from configured sources |
| `gitte list` | List all projects and their available actions |
| `gitte toggle` | Interactive TUI to enable/disable projects |
| `gitte features list` | List all feature gates and their enabled state |
| `gitte features enable <gate>` | Enable a feature gate |
| `gitte features disable <gate>` | Disable a feature gate |
| `gitte sources` | Manage local discovery sources (GitLab groups / GitHub orgs) |
| `gitte token set <gitlab\|github> <host>` | Store an API token in the system keyring |
| `gitte validate` | Validate config: schema, cycles, missing references |
| `gitte clean [flags]` | Report repo state (see below) |

### Argument syntax

Arguments to `run` and `actions` are positional: `action [group] [projects]`.

```bash
gitte run up                      # up action, all groups, all enabled projects
gitte run up local                # up action, group local, all enabled projects
gitte run up local myservice      # up action, group local, project myservice only
gitte run up local frontend+backend  # up action, group local, projects frontend and backend
gitte run up+build                # run up then build, all groups, all enabled projects
```

Use `*` or `all` as a wildcard. Combine multiple values with `+`.

### clean flags

Reports repos matching each condition — useful for spotting what needs attention.

```bash
gitte clean --untracked        # list repos with untracked files
gitte clean --local-changes    # list repos with local changes
gitte clean --master           # list repos on the default branch
gitte clean --non-gitte        # list directories not managed by gitte
```

## Configuration

See [docs/config.md](./docs/config.md) for the full configuration reference.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GITTE_CWD` | process cwd | Override the working directory |
| `GITTE_NO_TTY` | `0` | Set to `1` to force plain-text output (no TUI) |
| `GITTE_NO_NEEDS` | `0` | Set to `1` to ignore `needs` dependencies |
| `GITTE_NO_REBASE` | `false` | Set to `true` to skip auto-rebase onto default branch |
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
