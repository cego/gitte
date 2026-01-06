package config

import "context"

type GitteConfig struct {
	StartupChecks   StartupCheckMap           `yaml:"startup,omitempty"`
	Projects        map[string]ProjectConfig  `yaml:"projects,omitempty"`
	SearchFor       []SearchFor               `yaml:"searchFor,omitempty"`
	ActionOverrides map[string]ActionOverride `yaml:"actionOverride,omitempty"`
}

type ProjectConfig struct {
	Common          bool                     `yaml:"common"`
	Remote          string                   `yaml:"remote"`
	DefaultBranch   string                   `yaml:"default_branch"`
	Actions         map[string]ProjectAction `yaml:"actions"`
	DefaultDisabled bool                     `yaml:"defaultDisabled"`
}

type SearchFor struct {
	Regex string `yaml:"regex"`
	Hint  string `yaml:"hint"`
}

type ProjectAction struct {
	SearchFors []SearchFor         `yaml:"searchFor"`
	Priority   int                 `yaml:"priority"`
	Needs      []string            `yaml:"needs"`
	Groups     map[string][]string `yaml:"groups"`
}

type ActionOverride struct {
	maxParallelization int `yaml:"maxParallelization"`
}

const configContextKey = "gitteConfig"
const cwdContextKey = "cwd"

func ConfigFromContext(ctx context.Context) *GitteConfig {
	if cfg, ok := ctx.Value(configContextKey).(*GitteConfig); ok {
		return cfg
	}

	panic("gitte config not in context where expected. This is a bug.")
}

func ContextWithConfig(ctx context.Context, cfg *GitteConfig) context.Context {
	return context.WithValue(ctx, configContextKey, cfg)
}

func CwdFromContext(ctx context.Context) string {
	if cwd, ok := ctx.Value("cwd").(string); ok {
		return cwd
	}

	panic("cwd not in context where expected. This is a bug.")
}

func ContextWithCwd(ctx context.Context, cwd string) context.Context {
	return context.WithValue(ctx, cwdContextKey, cwd)
}
