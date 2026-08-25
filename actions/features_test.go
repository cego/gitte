package actions

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/state"
)

func TestEnabledFeaturesForProject(t *testing.T) {
	cfg := &config.GitteConfig{
		FeatureGates: map[string]config.FeatureGate{
			"feat-on":         {},                                                        // empty scope → applies to all projects
			"feat-off":        {},                                                        // disabled in state
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

func TestInjectedEnv(t *testing.T) {
	cfg := &config.GitteConfig{
		FeatureGates: map[string]config.FeatureGate{
			"feat-on": {Effects: config.FeatureEffects{Env: map[string]string{"FEAT_VAR": "1"}}},
		},
	}
	st := &state.GitteState{Features: map[string]state.FeatureState{"feat-on": {Enabled: true}}}
	proj := config.ProjectConfig{
		Remote: "git@github.com:example/myproj.git",
		Env:    map[string]string{"PROJ_VAR": "x"},
	}
	got := injectedEnv(cfg, st, "myproj", proj)
	if got["PROJ_VAR"] != "x" || got["FEAT_VAR"] != "1" {
		t.Fatalf("injectedEnv = %v, want PROJ_VAR=x and FEAT_VAR=1", got)
	}
}

func TestOutputTail(t *testing.T) {
	// stderr preferred when non-empty
	if got := outputTail([]byte("the real error"), []byte("noise")); got != "the real error" {
		t.Fatalf("stderr-preferred: got %q", got)
	}
	// falls back to stdout when stderr is blank (e.g. gitlab-ci-local prints to stdout)
	if got := outputTail([]byte("   \n"), []byte("stdout failure")); got != "stdout failure" {
		t.Fatalf("stdout-fallback: got %q", got)
	}
	// capped to the last errorTailBytes
	big := []byte(strings.Repeat("x", errorTailBytes+500))
	if got := outputTail(big, nil); len(got) != errorTailBytes {
		t.Fatalf("cap: len=%d, want %d", len(got), errorTailBytes)
	}
	// empty when no output
	if got := outputTail(nil, nil); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}
