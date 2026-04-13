package config

import (
	"testing"
)

func TestConfig_MergeOverride_ProjectsOverrideBase(t *testing.T) {
	base := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git"},
			"b": {Remote: "git@host:org/b.git"},
		},
	}
	override := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"b": {Remote: "git@host:org/b-override.git"},
			"c": {Remote: "git@host:org/c.git"},
		},
	}

	result := MergeOverride(base, override)

	if len(result.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(result.Projects))
	}
	if result.Projects["a"].Remote != "git@host:org/a.git" {
		t.Error("base-only project 'a' should be unchanged")
	}
	if result.Projects["b"].Remote != "git@host:org/b-override.git" {
		t.Error("override should win for project 'b'")
	}
	if result.Projects["c"].Remote != "git@host:org/c.git" {
		t.Error("override-only project 'c' should be added")
	}
}

func TestConfig_MergeOverride_NilOverrideReturnsBase(t *testing.T) {
	base := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git"},
		},
	}

	result := MergeOverride(base, nil)

	if result != base {
		t.Error("nil override should return base unchanged")
	}
}

func TestConfig_MergeOverride_NilBaseProjects(t *testing.T) {
	base := &GitteConfig{}
	override := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git"},
		},
	}

	result := MergeOverride(base, override)

	if len(result.Projects) != 1 || result.Projects["a"].Remote != "git@host:org/a.git" {
		t.Error("should handle nil base projects when override has projects")
	}
}

func TestConfig_WithTogglesApplied_DefaultEnabled(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git"},
			"b": {Remote: "git@host:org/b.git"},
		},
	}

	result := cfg.WithTogglesApplied(nil)

	if len(result.Projects) != 2 {
		t.Errorf("all projects should be included with no toggles, got %d", len(result.Projects))
	}
}

func TestConfig_WithTogglesApplied_ExplicitDisable(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git"},
			"b": {Remote: "git@host:org/b.git"},
		},
	}

	result := cfg.WithTogglesApplied(map[string]bool{"b": false})

	if _, ok := result.Projects["b"]; ok {
		t.Error("explicitly disabled project 'b' should be excluded")
	}
	if _, ok := result.Projects["a"]; !ok {
		t.Error("project 'a' should still be included")
	}
}

func TestConfig_WithTogglesApplied_DefaultDisabledRequiresExplicitEnable(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git", DefaultDisabled: true},
			"b": {Remote: "git@host:org/b.git", DefaultDisabled: true},
		},
	}

	// Enable only 'a'; 'b' has no toggle entry
	result := cfg.WithTogglesApplied(map[string]bool{"a": true})

	if _, ok := result.Projects["a"]; !ok {
		t.Error("explicitly enabled defaultDisabled project 'a' should be included")
	}
	if _, ok := result.Projects["b"]; ok {
		t.Error("defaultDisabled project 'b' with no toggle should be excluded")
	}
}

func TestConfig_WithTogglesApplied_DoesNotMutateReceiver(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {Remote: "git@host:org/a.git"},
			"b": {Remote: "git@host:org/b.git"},
		},
	}

	_ = cfg.WithTogglesApplied(map[string]bool{"b": false})

	if len(cfg.Projects) != 2 {
		t.Error("WithTogglesApplied should not mutate the receiver")
	}
}
