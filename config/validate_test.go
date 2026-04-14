package config

import (
	"strings"
	"testing"
)

func TestValidateConfig_ValidConfig(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"database": {
				Remote:        "git@github.com:example/database.git",
				DefaultBranch: "main",
			},
		},
	}

	result := ValidateConfig(cfg)
	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestValidateConfig_MissingRemote(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"database": {
				DefaultBranch: "main",
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.HasErrors() {
		t.Error("expected error for missing remote")
	}
}

func TestValidateConfig_CycleDetection(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {
				Remote: "git@github.com:example/a.git",
				Actions: map[string]ProjectAction{
					"up": {Needs: []string{"b"}},
				},
			},
			"b": {
				Remote: "git@github.com:example/b.git",
				Actions: map[string]ProjectAction{
					"up": {Needs: []string{"a"}},
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.HasErrors() {
		t.Error("expected cycle error")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "needs" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error in 'needs' field, got: %v", result.Errors)
	}
}

func TestValidateConfig_UnknownTemplateRef(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"a": {
				Remote:  "git@github.com:example/a.git",
				Extends: "nonexistent-template",
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.HasErrors() {
		t.Error("expected error for unknown template reference")
	}
}

func TestValidateConfig_EnvWhenUnknownConditionType(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"myservice": {
				Remote: "git@github.com:example/myservice.git",
				EnvWhen: map[string]EnvWhenEntry{
					"FOO": {
						Value: "bar",
						Conditions: []EnvWhenCondition{
							{Type: "not_a_real_type"},
						},
					},
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.HasErrors() {
		t.Fatal("expected validation error for unknown condition type")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Field, "env_when") && strings.Contains(e.Message, "not_a_real_type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error mentioning env_when and 'not_a_real_type', got: %v", result.Errors)
	}
}

func TestValidateConfig_EnvWhenValidArchType(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"myservice": {
				Remote: "git@github.com:example/myservice.git",
				EnvWhen: map[string]EnvWhenEntry{
					"FOO": {
						Value: "bar",
						Conditions: []EnvWhenCondition{
							{Type: "arch", Arch: []string{"amd64"}},
						},
					},
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	if result.HasErrors() {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestValidateConfig_EnvWhenFeatureGateUnknownConditionType(t *testing.T) {
	cfg := &GitteConfig{
		FeatureGates: map[string]FeatureGate{
			"my-feature": {
				Effects: FeatureEffects{
					EnvWhen: map[string]EnvWhenEntry{
						"FOO": {
							Value: "bar",
							Conditions: []EnvWhenCondition{
								{Type: "not_a_real_type"},
							},
						},
					},
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.HasErrors() {
		t.Fatal("expected validation error for unknown condition type in feature gate")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Field, "env_when") && strings.Contains(e.Message, "not_a_real_type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error mentioning env_when and 'not_a_real_type', got: %v", result.Errors)
	}
}
