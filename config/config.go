package config

type GitteConfig struct {
	StartupChecks   map[string]StartupCheck   `yaml:"startup,omitempty"`
	Projects        map[string]ProjectConfig  `yaml:"projects,omitempty"`
	SearchFor       []SearchFor               `yaml:"searchFor,omitempty"`
	ActionOverrides map[string]ActionOverride `yaml:"actionOverride,omitempty"`
}

type StartupCheck struct {
	// TODO
	//export type Action = {
	//	hint?: string;
	//};
	//export type CmdAction = Action & { cmd: [string, ...string[]] };
	//export type ShellAction = Action & { shell: string; script: string };

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
