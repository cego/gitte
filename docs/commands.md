# Commands

All commands support the global flags `--config <path>`, `--cwd <path>`, and `--no-tty`.

---

## gitte run

Full pipeline: startup checks → git sync → actions.

```bash
gitte run up
gitte run up local
gitte run up local myservice
gitte run up local frontend+backend
gitte run up+build
gitte run up --discover    # also fetch repos from configured sources first
```

Arguments:
1. **action** — action name(s), `+`-separated. Required.
2. **group** — group name or `*` for all. Optional, defaults to all.
3. **projects** — project name(s) or `*` for all. Optional, defaults to all enabled.

---

## gitte actions

Run actions only, skipping startup checks and git sync. Same argument syntax as `run`.

```bash
gitte actions up
gitte actions up local
gitte actions down local myservice
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

Destructive cleanup operations on project repositories. All subcommands show a live progress TUI while running (plain-text lines in non-TTY mode).

```bash
gitte clean untracked       # run git clean -fdx in every repo
gitte clean local-changes   # run git reset --hard in repos with local changes (prompts first)
gitte clean master          # run git checkout <default_branch> in every repo
gitte clean all             # run untracked → local-changes → master in sequence
```

`gitte clean local-changes` shows all affected repos, then prompts for a single keypress:

```
Reset [a]ll / [i]ndividually / [C]ancel?
```

If `i` (individually), you are prompted per repo with a single keypress: `Reset <name>? [y/N]`

Operations run on all configured projects regardless of toggle state.

---

## gitte list

List all enabled projects and their available actions and groups.

```bash
gitte list        # enabled projects only
gitte list -a     # include disabled projects
```

---

## gitte sources

Manage local discovery sources — the GitLab groups and GitHub orgs that `gitte gitops --discover` queries. Sources are stored in `.gitte-override.yml` so they stay local to your machine.

```bash
gitte sources                                          # list configured sources
gitte sources add gitlab gitlab.example.com mygroup    # add a GitLab group
gitte sources add github github.com myorg              # add a GitHub org
gitte sources remove gitlab gitlab.example.com mygroup # remove a group
```

Tokens for discovery are looked up from the system keyring automatically. See `gitte token`.

---

## gitte token

Store and retrieve API tokens for GitLab and GitHub hosts in the system keyring (macOS Keychain or GNOME Keyring on Linux).

```bash
gitte token set gitlab gitlab.example.com   # store a token (prompts for input)
gitte token set github github.com
gitte token get gitlab gitlab.example.com   # print the stored token (diagnostic)
gitte token delete gitlab gitlab.example.com
gitte token list                            # show keyring status for all configured sources
```

Tokens are used automatically during `gitte gitops --discover`. If the keyring is unavailable (headless servers), use `token_env` or `token_cmd` in the source config instead.
