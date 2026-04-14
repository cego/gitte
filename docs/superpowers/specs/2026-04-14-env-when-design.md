# `env_when` — Conditional Environment Variables

**Goal:** Allow environment variables in templates, projects, and feature gate effects to be conditionally injected based on runtime properties of the machine (initially: CPU architecture).

**Architecture:** A new `env_when` key sits alongside the existing `env` key in templates, projects, and feature gate effects. Each entry declares a value and a list of conditions that must all pass for the variable to be injected. Condition evaluation happens at action run time using a new `config.ResolveEnvWhen` helper, which is called from `actions/runner.go` alongside the existing `env` resolution.

**Tech stack:** Go `runtime.GOARCH` for arch detection; no new dependencies.

---

## Configuration schema

### New types in `config/types.go`

```go
// EnvWhenEntry is one entry in an env_when map.
type EnvWhenEntry struct {
    Value      string           `yaml:"value"`
    Conditions []EnvWhenCondition `yaml:"conditions"`
}

// EnvWhenCondition is a single condition. Only fields relevant to the type are used.
type EnvWhenCondition struct {
    Type string   `yaml:"type"`           // currently only "arch"
    Arch []string `yaml:"arch,omitempty"` // used when type == "arch"
}
```

### `env_when` added to `Template`, `ProjectConfig`, and `FeatureEffects`

```go
type Template struct {
    // ... existing fields ...
    EnvWhen map[string]EnvWhenEntry `yaml:"env_when,omitempty"`
}

type ProjectConfig struct {
    // ... existing fields ...
    EnvWhen map[string]EnvWhenEntry `yaml:"env_when,omitempty"`
}

type FeatureEffects struct {
    Env     map[string]string       `yaml:"env,omitempty"`
    EnvWhen map[string]EnvWhenEntry `yaml:"env_when,omitempty"`
}
```

### Example `.gitte.yml` usage

```yaml
templates:
  sn-project:
    env:
      SOME_UNCONDITIONAL_VAR: "value"
    env_when:
      BUILD_FROM_SOURCE:
        value: "false"
        conditions:
          - type: arch
            arch: [amd64]
      GCL_VARIABLE_BUILD_FROM_SOURCE:
        value: "false"
        conditions:
          - type: arch
            arch: [amd64]

feature_gates:
  BUILD_FROM_SOURCE:
    description: "Build from source instead of using latest production tag"
    effects:
      env_when:
        BUILD_FROM_SOURCE:
          value: "true"
          conditions:
            - type: arch
              arch: [amd64]
        GCL_VARIABLE_BUILD_FROM_SOURCE:
          value: "true"
          conditions:
            - type: arch
              arch: [amd64]
```

---

## Condition evaluation

Conditions within one `env_when` entry are **ANDed** — all must pass for the variable to be injected. If `conditions` is empty, the entry is always injected (equivalent to plain `env`).

Currently supported condition types:

| `type` | Field | Behaviour |
|--------|-------|-----------|
| `arch` | `arch: [amd64, arm64, ...]` | Pass if `runtime.GOARCH` is in the list |

When a condition is not met, the variable is **silently skipped** — no warning, no log line.

---

## File structure

| File | Change |
|------|--------|
| `config/types.go` | Add `EnvWhenEntry`, `EnvWhenCondition`; add `EnvWhen` field to `Template`, `ProjectConfig`, `FeatureEffects` |
| `config/env_when.go` | New file: `ResolveEnvWhen(entries map[string]EnvWhenEntry) map[string]string` |
| `config/template.go` | Merge template `EnvWhen` into resolved project (same pattern as `Env`) |
| `actions/runner.go` | Call `config.ResolveEnvWhen` in `buildEnv` and `extraEnvForProject` |
| `config/validate.go` | Validate known condition `type` values, report unknown types |

---

## Env resolution priority (low → high)

1. OS environment
2. Project/template `env` (unconditional)
3. Project/template `env_when` (conditional, same layer as `env`)
4. Feature gate `effects.env` (unconditional)
5. Feature gate `effects.env_when` (conditional, same layer as gate `env`)

---

## Template resolution

`env_when` entries from a template are merged into the resolved project's `env_when`, following the same merge semantics as `env`: the project's own `env_when` entries win over the template's for conflicting keys.

---

## Validation

`gitte validate` will report an error for any `env_when` condition with an unknown `type`. Currently the only valid type is `arch`. This prevents silent misconfiguration when new condition types are added later.

---

## Future condition types

The `conditions` list structure allows adding new types without breaking existing configs:

```yaml
conditions:
  - type: arch
    arch: [amd64]
  - type: os
    os: [linux, darwin]
```

No other types are implemented in this iteration.
