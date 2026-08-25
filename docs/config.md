# Configuration reference

Gitte is configured with a `.gitte.yml` file. Gitte walks up from the current directory to find it, so you can run gitte from any subdirectory of your workspace.

An optional `.gitte-override.yml` in the same directory is deep-merged on top, useful for local machine-specific overrides that should not be committed.

---

## Table of contents

- [Top-level structure](#top-level-structure)
- [startup](#startup)
- [projects](#projects)
- [templates](#templates)
- [groupIncludes](#groupincludes)
- [retry](#retry)
- [actionOverride](#actionoverride)
- [searchFor](#searchfor)
- [feature\_gates](#feature_gates)
- [sources](#sources)
- [telemetry](#telemetry)
- [Remote configuration](#remote-configuration)

---

## Top-level structure

```yaml
startup:        # startup checks (optional)
templates:      # reusable project templates (optional)
projects:       # project definitions (required)
groupIncludes:  # group inclusion rules — running group X also runs group Y (optional)
feature_gates:  # feature gates (optional)
sources:        # auto-discovery sources (optional)
searchFor:      # global output pattern matching (optional)
actionOverride: # per-action overrides (optional)
retry:          # global retry defaults (optional)
telemetry:      # OpenTelemetry trace export (optional)
```

---

## startup

Startup checks run before anything else when using `gitte run` or `gitte startup`. If any check fails, gitte exits and prints the hint.

Checks support `needs` for ordering (e.g. check Docker version only after confirming Docker is installed).

### type: command

Runs a command and checks the exit code.

```yaml
startup:
  git-present:
    type: command
    cmd: ["git", "--version"]
    hint: "git is not installed"

  docker-present:
    type: command
    cmd: ["docker", "--version"]
    hint: "Docker is not installed"

  docker-version:
    type: shell
    shell: bash
    needs: [docker-present]
    script: |
      current="$(docker --version | grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
      minimum="25.0.0"
      [ "$current" = "$(printf '%s\n%s' "$current" "$minimum" | sort -V | tail -1)" ]
    hint: "Docker must be at least version 25.0.0"
```

### type: shell

Runs a multi-line shell script. The `shell` field specifies the interpreter (e.g. `bash`, `sh`).

```yaml
startup:
  check-network:
    type: shell
    shell: bash
    script: |
      docker network ls | grep -q mynetwork || \
        docker network create mynetwork --driver overlay --attachable
    hint: "Failed to ensure Docker network exists"
```

### type: yaml-path-present

Checks that a key exists (and is non-empty) in a YAML file. Supports `~/` path expansion. Uses JSONPath-style syntax for the `path` field.

```yaml
startup:
  check-token:
    type: yaml-path-present
    file: "~/.config/mytool/credentials.yml"
    path: "$.tokens.registry"
    hint: "Registry token missing from ~/.config/mytool/credentials.yml"
```

---

## projects

Each entry under `projects` defines a repository gitte manages.

```yaml
projects:
  myservice:
    remote: git@github.com:example/myservice.git
    default_branch: main
    defaultDisabled: false   # if true, project is off unless explicitly enabled
    extends: my-template     # inherit from a template (optional)
    vars:                    # template variable overrides (optional)
      stack_name: myservice-custom
    actions:
      start:
        needs: [database]    # run after database:start completes
        retry:
          attempts: 2
          delay: 10s
          backoff: exponential
        groups:
          prod: ["docker", "stack", "deploy", "myservice-prod"]
          staging: ["docker", "compose", "up", "-d"]
      stop:
        groups:
          prod: ["docker", "stack", "rm", "myservice-prod"]
          staging: ["docker", "compose", "down"]
      test:
        groups:
          "*": ["make", "test"]   # wildcard group matches any group argument
```

### Remote URL formats

Both SSH and HTTPS are supported. The local directory is derived from the remote URL:

```
git@github.com:example/myservice.git     →  github.com/example/myservice
git@gitlab.example.com:org/svc/api.git   →  gitlab.example.com/org/svc/api
https://github.com/example/myservice.git →  github.com/example/myservice
```

### actions

Each action maps group names to commands. When you run `gitte run start prod`, gitte executes the command under `groups.prod` for each enabled project that has a `start` action.

**needs** — list of project names that must complete this action successfully before this project starts. Gitte resolves the full dependency graph and runs independent tasks in parallel.

**retry** — retry the action on failure (see [retry](#retry)).

**searchFor** — per-action output pattern matching (see [searchFor](#searchfor)).

---

## templates

Templates let you define shared action sets and reuse them across many similar projects. A project opts in with `extends: <template-name>`.

Template variables are rendered using Go `text/template` syntax. The following variables are always available:

| Variable | Value |
|----------|-------|
| `{{.project}}` | the project key |
| `{{.remote}}` | the project's remote URL |
| any key from `vars` | the resolved variable value |

```yaml
templates:
  docker-service:
    vars:
      stack: "{{.project}}-prod"
    actions:
      start:
        groups:
          prod: ["docker", "stack", "deploy", "{{.stack}}"]
          staging: ["docker", "compose", "up", "-d"]
      stop:
        groups:
          prod: ["docker", "stack", "rm", "{{.stack}}"]
          staging: ["docker", "compose", "down"]

projects:
  frontend:
    remote: git@github.com:example/frontend.git
    default_branch: main
    extends: docker-service          # inherits all actions

  backend:
    remote: git@github.com:example/backend.git
    default_branch: main
    extends: docker-service
    vars:
      stack: "backend-custom-prod"   # override specific variable
    actions:
      start:
        needs: [database]            # add dependency on top of template
```

When both the template and the project define the same action, the project's group commands take precedence for matching group keys. The project can also add or replace `needs`.

### Template inheritance

Templates can themselves extend other templates using `extends`. This allows building a hierarchy of shared definitions.

```yaml
templates:
  base-service:
    vars:
      stack: "{{.project}}"
    actions:
      stop:
        groups:
          prod: ["docker", "stack", "rm", "{{.stack}}"]

  php-service:
    extends: [base-service]    # inherits all of base-service
    actions:
      start:
        needs: [database]

  full-service:
    extends: [base-service, php-service]   # merge multiple parents left-to-right
```

Multiple parents are merged left-to-right; the rightmost definition wins for conflicting keys. The template's own definitions are applied last.

---

## groupIncludes

`groupIncludes` lets you declare that running one group should automatically include the tasks of another group. This is useful for shared infrastructure that multiple teams need but should not appear under a wildcard `*` group.

```yaml
groupIncludes:
  team-a: [shared]    # running group "team-a" also runs group "shared"
  team-b: [shared]    # running group "team-b" also runs group "shared"
  shared: [infra]     # transitive: "team-a" and "team-b" also pull in "infra"
```

Expansion is **transitive** — if `team-a` includes `shared` and `shared` includes `infra`, then running group `team-a` automatically includes both `shared` and `infra` tasks.

This replaces the need for `*` wildcard groups on infrastructure projects. Give an infrastructure project a specific group name and list it in `groupIncludes` for the teams that need it.

---

## retry

Configure how failed tasks are retried.

**Global default** (applies to all actions unless overridden):

```yaml
retry:
  default:
    attempts: 2
    delay: 5s
    backoff: linear
```

**Per-action** (overrides the global default for that action):

```yaml
projects:
  myservice:
    actions:
      start:
        retry:
          attempts: 3
          delay: 10s
          backoff: exponential
```

| Field | Values | Description |
|-------|--------|-------------|
| `attempts` | integer ≥ 1 | Total attempts (1 = no retry) |
| `delay` | e.g. `5s`, `30s` | Base delay between attempts |
| `backoff` | `none`, `linear`, `exponential` | How delay grows with each attempt |

Backoff modes:
- `none` — always wait `delay`
- `linear` — wait `delay × attempt`
- `exponential` — wait `delay × 2^attempt`

---

## actionOverride

Override per-action settings globally.

```yaml
actionOverride:
  stop:
    maxParallelization: 1   # run stop actions one at a time
```

---

## searchFor

Scan action output for regex patterns and display a hint when matched. Useful for surfacing common errors with actionable messages.

```yaml
searchFor:
  - regex: "authentication required"
    hint: "Registry login expired — run: docker login registry.example.com"
  - regex: "connection refused"
    hint: "Service may not be running yet, try again in a moment"
```

Can also be defined per-action:

```yaml
projects:
  myservice:
    actions:
      start:
        searchFor:
          - regex: "port already in use"
            hint: "Port conflict — check for other running services"
```

---

## feature_gates

Feature gates let individual developers enable opt-in behaviours on their machine. When enabled, the gate injects environment variables into matching action executions.

```yaml
feature_gates:
  HOT_RELOAD:
    description: "Enable hot reload for frontend development"
    effects:
      env:
        VITE_HMR: "true"
        HOT_RELOAD: "true"
    scope:
      projects: [frontend, admin-ui]
```

Scope can target projects by name, by GitLab group, or by GitHub org:

```yaml
feature_gates:
  DEBUG_MODE:
    effects:
      env:
        DEBUG: "1"
    scope:
      projects: [myservice]
      gitlab_groups:
        - host: gitlab.example.com
          group: myorg/services
      github_orgs:
        - host: github.com
          org: myorg
```

Manage feature gates with:

```bash
gitte features list
gitte features enable HOT_RELOAD
gitte features disable HOT_RELOAD
```

`enable` and `disable` both accept `--project`, `--gitlab-group`, and `--github-org`
flags to target a subset. A scoped `disable` removes only the matching projects from the
gate's scope, leaving it on for the rest:

```bash
gitte features enable HOT_RELOAD --project frontend
gitte features disable HOT_RELOAD --project frontend
```

---

## sources

Configure auto-discovery of repositories from GitLab groups or GitHub orgs. Discovered repos are cloned/pulled but not written to `.gitte.yml`.

```yaml
sources:
  gitlab:
    - host: gitlab.example.com
      token_env: GITLAB_TOKEN      # env var containing the API token
      groups:
        - myorg/services
        - myorg/tools

  github:
    - host: github.com
      token_env: GITHUB_TOKEN
      orgs:
        - myorg
```

Run discovery with:

```bash
gitte gitops --discover
gitte run start --discover    # discover, then sync, then run actions
```

---

## telemetry

Gitte can export OpenTelemetry traces over OTLP/HTTP to an OTLP-compatible backend (e.g. Elastic APM) to help debug failures. Telemetry is enabled whenever an endpoint is resolved.

```yaml
telemetry:
  endpoint: https://apm.example.com:8200   # OTLP/HTTP endpoint
  headers:                                  # arbitrary export headers (optional)
    Authorization: "Bearer <secret-token>"  # or: "ApiKey <base64-key>"
```

| Field | Description |
|-------|-------------|
| `endpoint` | OTLP/HTTP endpoint to export spans to. Telemetry is enabled when this resolves to a non-empty value. |
| `headers` | Map of HTTP headers attached to every export request — typically authentication (`Authorization`). |

Each invocation produces one trace: a root span for the command, child spans for each repo sync (branch, commit SHA, dirty flag) and each action task (command, exit code), with errors recorded on the relevant span. The OS username (`user.name`) and hostname (`host.name`) are attached to every trace to identify which developer and machine produced it.

The gitte CLI arguments and each action's command line are exported as span attributes. gitte's injected environment — a project's `env`, `env_when`, and feature-gate env — is also exported (the `gitte.env` span attribute and the task logs); the inherited process environment (`os.Environ`) is not. Keep secrets out of action command definitions, CLI arguments, and config `env`/feature-gate blocks — pass real secrets through the process environment instead. Full remote URLs are never collected (repos are identified by name only).

Environment variables override or disable telemetry:

| Variable | Effect |
|----------|--------|
| `GITTE_TELEMETRY=off`, `false`, or `0` | Disable telemetry locally (case-insensitive; surrounding whitespace is ignored) |
| `GITTE_TELEMETRY_URL` | Override the endpoint; whitespace is trimmed and `https://` is added when no scheme is present |
| `GITTE_TELEMETRY_LOGS=off`, `false`, or `0` | Disable OTEL log export (keep traces; case-insensitive with surrounding whitespace ignored) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_EXPORTER_OTLP_HEADERS` | Standard OTEL env vars; endpoint vars are used when no gitte endpoint is set, and headers can provide credentials for a `GITTE_TELEMETRY_URL` override |

Precedence: `GITTE_TELEMETRY=off|false|0` > `GITTE_TELEMETRY_URL` > config `endpoint` > `OTEL_EXPORTER_OTLP_*`. Telemetry is best-effort and never blocks or slows gitte; export failures are silently ignored and flushing on exit is time-bounded.

When `GITTE_TELEMETRY_URL` overrides the config endpoint, config headers are not
used. Set `OTEL_EXPORTER_OTLP_HEADERS` to provide credentials for the override
endpoint through the OTEL exporter.

Action and startup command output is also exported as OTEL logs (correlated to
the producing span) unless `GITTE_TELEMETRY_LOGS` is `off`, `false`, or `0`.

---

## Remote configuration

Gitte can load its configuration from a remote git repository. Create a `.gitte-env` file alongside `.gitte.yml`:

```
REMOTE_GIT_REPO="git@github.com:example/gitte-config.git"
REMOTE_GIT_FILE=".gitte.yml"
REMOTE_GIT_REF="main"
```

Gitte fetches the file using `git archive` and caches it in `.gitte-state.yml`. The cache is refreshed in the background on each run.
