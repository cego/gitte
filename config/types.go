package config

import (
	"context"
)

// GitteConfig is the top-level configuration struct for .gitte.yml
type GitteConfig struct {
	StartupChecks  StartupCheckMap           `yaml:"startup,omitempty"`
	Templates      map[string]Template       `yaml:"templates,omitempty"`
	FeatureGates   map[string]FeatureGate    `yaml:"feature_gates,omitempty"`
	Sources        Sources                   `yaml:"sources,omitempty"`
	SearchFor      []SearchFor               `yaml:"searchFor,omitempty"`
	ActionOverride map[string]ActionOverride `yaml:"actionOverride,omitempty"`
	Retry          RetryDefaults             `yaml:"retry,omitempty"`
	GroupIncludes  map[string][]string       `yaml:"groupIncludes,omitempty"`
	Projects       map[string]ProjectConfig  `yaml:"projects,omitempty"`
	QuickSolve     QuickSolveConfig          `yaml:"quickSolve,omitempty"`
}

// QuickSolveConfig holds settings for the quick solve feature in the actions TUI.
type QuickSolveConfig struct {
	GitClean QuickSolveGitClean `yaml:"git_clean,omitempty"`
}

// QuickSolveGitClean configures which directories are excluded from git clean -fdx.
type QuickSolveGitClean struct {
	Exclude []string `yaml:"exclude,omitempty"`
}

// Template is a reusable project configuration template.
// Extends lists parent template names (resolved left-to-right, self applied last).
type Template struct {
	Extends []string                 `yaml:"extends,omitempty"`
	Vars    map[string]string        `yaml:"vars,omitempty"`
	Actions map[string]ProjectAction `yaml:"actions,omitempty"`
}

// ProjectConfig represents a single project in the config
type ProjectConfig struct {
	Remote          string                   `yaml:"remote"`
	DefaultBranch   string                   `yaml:"default_branch,omitempty"`
	Actions         map[string]ProjectAction `yaml:"actions,omitempty"`
	DefaultDisabled bool                     `yaml:"defaultDisabled,omitempty"`
	Extends         string                   `yaml:"extends,omitempty"`
	Vars            map[string]string        `yaml:"vars,omitempty"`
}

// ProjectAction represents an action (build/up/down/purge) for a project
type ProjectAction struct {
	SearchFors []SearchFor         `yaml:"searchFor,omitempty"`
	Needs      []string            `yaml:"needs,omitempty"`
	Groups     map[string][]string `yaml:"groups,omitempty"`
	Retry      *RetryConfig        `yaml:"retry,omitempty"`
}

// SearchFor defines a regex pattern and hint for output matching
type SearchFor struct {
	Regex string `yaml:"regex"`
	Hint  string `yaml:"hint"`
}

// ActionOverride allows per-action configuration overrides
type ActionOverride struct {
	MaxParallelization int `yaml:"maxParallelization,omitempty"`
}

// RetryConfig defines retry behavior for a task
type RetryConfig struct {
	Attempts int    `yaml:"attempts"`
	Delay    string `yaml:"delay,omitempty"`   // e.g. "5s", "10s"
	Backoff  string `yaml:"backoff,omitempty"` // "none", "linear", "exponential"
}

// RetryDefaults holds global retry defaults
type RetryDefaults struct {
	Default RetryConfig `yaml:"default,omitempty"`
}

// FeatureGate defines a feature that can be enabled/disabled per machine
type FeatureGate struct {
	Description string         `yaml:"description,omitempty"`
	Effects     FeatureEffects `yaml:"effects,omitempty"`
	Scope       FeatureScope   `yaml:"scope,omitempty"`
}

// FeatureEffects defines what a feature gate does when enabled
type FeatureEffects struct {
	Env map[string]string `yaml:"env,omitempty"`
}

// FeatureScope defines which projects a feature gate applies to
type FeatureScope struct {
	Projects     []string      `yaml:"projects,omitempty"`
	GitlabGroups []GitlabScope `yaml:"gitlab_groups,omitempty"`
	GithubOrgs   []GithubScope `yaml:"github_orgs,omitempty"`
}

// GitlabScope defines a GitLab group scope for feature gates
type GitlabScope struct {
	Host  string `yaml:"host"`
	Group string `yaml:"group"`
}

// GithubScope defines a GitHub org scope for feature gates
type GithubScope struct {
	Host string `yaml:"host"`
	Org  string `yaml:"org"`
}

// Sources defines auto-discovery sources
type Sources struct {
	Gitlab []GitlabSource `yaml:"gitlab,omitempty"`
	Github []GithubSource `yaml:"github,omitempty"`
}

// GitlabSource defines a GitLab source for auto-discovery
type GitlabSource struct {
	Host     string   `yaml:"host"`
	TokenEnv string   `yaml:"token_env"`
	Groups   []string `yaml:"groups,omitempty"`
}

// GithubSource defines a GitHub source for auto-discovery
type GithubSource struct {
	Host     string   `yaml:"host"`
	TokenEnv string   `yaml:"token_env"`
	Orgs     []string `yaml:"orgs,omitempty"`
}

type contextKey string

const configContextKey contextKey = "gitteConfig"
const cwdContextKey contextKey = "cwd"

// ConfigFromContext retrieves the GitteConfig from context
func ConfigFromContext(ctx context.Context) *GitteConfig {
	if cfg, ok := ctx.Value(configContextKey).(*GitteConfig); ok {
		return cfg
	}
	panic("gitte config not in context where expected. This is a bug.")
}

// ContextWithConfig stores the GitteConfig in context
func ContextWithConfig(ctx context.Context, cfg *GitteConfig) context.Context {
	return context.WithValue(ctx, configContextKey, cfg)
}

// CwdFromContext retrieves the working directory from context
func CwdFromContext(ctx context.Context) string {
	if cwd, ok := ctx.Value(cwdContextKey).(string); ok {
		return cwd
	}
	panic("cwd not in context where expected. This is a bug.")
}

// ContextWithCwd stores the working directory in context
func ContextWithCwd(ctx context.Context, cwd string) context.Context {
	return context.WithValue(ctx, cwdContextKey, cwd)
}

// ToggledProjects maps project keys to their explicit enabled/disabled state
type ToggledProjects = map[string]bool

// FilterToggles removes projects based on toggle state
func (cfg *GitteConfig) FilterToggles(toggledProjects ToggledProjects) {
	filteredProjects := make(map[string]ProjectConfig)
	for key, project := range cfg.Projects {
		enabled, isToggled := toggledProjects[key]
		if project.DefaultDisabled {
			if isToggled && enabled {
				filteredProjects[key] = project
			}
		} else {
			if !isToggled || enabled {
				filteredProjects[key] = project
			}
		}
	}
	cfg.Projects = filteredProjects
}

// WithTogglesApplied returns a shallow copy of the config with toggles applied,
// leaving the receiver unmodified. Used by the toggle command which needs the
// full (unfiltered) project list.
func (cfg *GitteConfig) WithTogglesApplied(toggledProjects ToggledProjects) *GitteConfig {
	filtered := make(map[string]ProjectConfig, len(cfg.Projects))
	for key, project := range cfg.Projects {
		enabled, isToggled := toggledProjects[key]
		if project.DefaultDisabled {
			if isToggled && enabled {
				filtered[key] = project
			}
		} else {
			if !isToggled || enabled {
				filtered[key] = project
			}
		}
	}
	copy := *cfg
	copy.Projects = filtered
	return &copy
}
