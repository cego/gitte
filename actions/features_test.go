package actions

import (
	"reflect"
	"testing"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/state"
)

func TestEnabledFeaturesForProject(t *testing.T) {
	cfg := &config.GitteConfig{
		FeatureGates: map[string]config.FeatureGate{
			"feat-on":         {},                                                          // empty scope → applies to all projects
			"feat-off":        {},                                                          // disabled in state
			"feat-scoped-out": {Scope: config.FeatureScope{Projects: []string{"other"}}}, // enabled but scoped to a different project
		},
	}
	st := &state.GitteState{Features: map[string]state.FeatureState{
		"feat-on":         {Enabled: true},
		"feat-off":        {Enabled: false},
		"feat-scoped-out": {Enabled: true},
	}}
	proj := config.ProjectConfig{Remote: "git@github.com:example/myproj.git"}

	got := enabledFeaturesForProject(cfg, st, "myproj", proj)
	want := []string{"feat-on"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enabledFeaturesForProject = %v, want %v", got, want)
	}
}
