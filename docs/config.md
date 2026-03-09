# Configuration reference

Gitte is configured with a `.gitte.yml` file. Gitte walks up from the current directory to find it, so you can run gitte from any subdirectory of your workspace.

An optional `.gitte-override.yml` in the same directory is deep-merged on top, useful for local machine-specific overrides that should not be committed.

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
      up:
        needs: [database]    # run after database:up completes
        retry:
          attempts: 2
          delay: 10s
          backoff: exponential
        groups:
          prod: ["docker", "stack", "deploy", "myservice-prod"]
          staging: ["docker", "compose", "up", "-d"]
      down:
        groups:
          prod: ["docker", "stack", "rm", "myservice-prod"]
          staging: ["docker", "compose", "down"]
      build:
        groups:
          "*": ["make", "build"]   # wildcard group matches any group argument
```

### Remote URL formats

Both SSH and HTTPS are supported. The local directory is derived from the remote URL:

```
git@github.com:example/myservice.git     →  github.com/example/myservice
git@gitlab.example.com:org/svc/api.git   →  gitlab.example.com/org/svc/api
https://github.com/example/myservice.git →  github.com/example/myservice
```

### actions

Each action maps group names to commands. When you run `gitte run up prod`, gitte executes the command under `groups.prod` for each enabled project that has an `up` action.

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
      up:
        groups:
          prod: ["docker", "stack", "deploy", "{{.stack}}"]
          staging: ["docker", "compose", "up", "-d"]
      down:
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
      up:
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
      down:
        groups:
          prod: ["docker", "stack", "rm", "{{.stack}}"]

  php-service:
    extends: [base-service]    # inherits all of base-service
    actions:
      up:
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
  sn: [cego]      # running group "sn" also runs group "cego"
  ht: [cego]      # running group "ht" also runs group "cego"
  cego: [streaming]   # transitive: "sn" and "ht" also pull in "streaming"
```

Expansion is **transitive** — if `sn` includes `cego` and `cego` includes `streaming`, then running group `sn` automatically includes both `cego` and `streaming` tasks.

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
      up:
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
  down:
    maxParallelization: 1   # run down actions one at a time
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
      up:
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
gitte run up --discover    # discover, then sync, then run actions
```

---

## Remote configuration

Gitte can load its configuration from a remote git repository. Create a `.gitte-env` file alongside `.gitte.yml`:

```
REMOTE_GIT_REPO="git@github.com:example/gitte-config.git"
REMOTE_GIT_FILE=".gitte.yml"
REMOTE_GIT_REF="main"
```

Gitte fetches the file using `git archive` and caches it in `.gitte-state.yml`. The cache is refreshed in the background on each run.
