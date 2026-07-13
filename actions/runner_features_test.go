package actions

import (
	"testing"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/state"
)

// TestExtraEnvForProject_OverrideCannotBroadenConfigScope guards against a scoped
// `features disable` (or the TUI scope editor) reconstructing an OverrideScope that
// matches projects outside the gate's configured scope. The override may only narrow
// the config scope, never broaden it.
func TestExtraEnvForProject_OverrideCannotBroadenConfigScope(t *testing.T) {
	cfg := &config.GitteConfig{
		FeatureGates: map[string]config.FeatureGate{
			"hot": {
				Scope:   config.FeatureScope{Projects: []string{"svc-a", "svc-b"}},
				Effects: config.FeatureEffects{Env: map[string]string{"FOO": "bar"}},
			},
		},
		Projects: map[string]config.ProjectConfig{
			"svc-a": {Remote: "git@gitlab.example.com:myorg/services/svc-a.git"},
			"svc-b": {Remote: "git@gitlab.example.com:myorg/services/svc-b.git"},
			"other": {Remote: "git@gitlab.example.com:myorg/tools/other.git"},
		},
	}

	// Simulates the state after `disable hot --project svc-a`: the reconstruction
	// collapses the remaining projects into the top-level "myorg" group excluding
	// svc-a — a group that also covers myorg/tools/other, which was never in scope.
	st := &state.GitteState{
		Features: map[string]state.FeatureState{
			"hot": {
				Enabled: true,
				OverrideScope: &state.ScopeOverride{
					GitlabGroups: []state.ScopeOverrideGroup{
						{Host: "gitlab.example.com", Group: "myorg", ExcludeProjects: []string{"svc-a"}},
					},
				},
			},
		},
	}

	cases := []struct {
		proj    string
		wantEnv bool
	}{
		{"svc-b", true},  // still in scope and not excluded
		{"svc-a", false}, // in config scope but excluded by the override
		{"other", false}, // NEVER in config scope — must not be broadened in
	}

	for _, tc := range cases {
		env := extraEnvForProject(cfg, st, tc.proj, cfg.Projects[tc.proj])
		if got := env["FOO"] == "bar"; got != tc.wantEnv {
			t.Errorf("%s: got env=%v, want %v (env=%v)", tc.proj, got, tc.wantEnv, env)
		}
	}
}
