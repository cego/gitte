package config

import (
	"testing"
)

func TestResolveEnvWhen_EmptyEntries(t *testing.T) {
	result := ResolveEnvWhen(nil, "amd64")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestResolveEnvWhen_NoConditions_AlwaysIncluded(t *testing.T) {
	entries := map[string]EnvWhenEntry{
		"FOO": {Value: "bar"},
	}
	result := ResolveEnvWhen(entries, "arm64")
	if result["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got %v", result)
	}
}

func TestResolveEnvWhen_ArchMatch_Included(t *testing.T) {
	entries := map[string]EnvWhenEntry{
		"BUILD_FROM_SOURCE": {
			Value: "false",
			Conditions: []EnvWhenCondition{
				{Type: "arch", Arch: []string{"amd64"}},
			},
		},
	}
	result := ResolveEnvWhen(entries, "amd64")
	if result["BUILD_FROM_SOURCE"] != "false" {
		t.Errorf("expected BUILD_FROM_SOURCE=false, got %v", result)
	}
}

func TestResolveEnvWhen_ArchNoMatch_Excluded(t *testing.T) {
	entries := map[string]EnvWhenEntry{
		"BUILD_FROM_SOURCE": {
			Value: "false",
			Conditions: []EnvWhenCondition{
				{Type: "arch", Arch: []string{"amd64"}},
			},
		},
	}
	result := ResolveEnvWhen(entries, "arm64")
	if _, ok := result["BUILD_FROM_SOURCE"]; ok {
		t.Errorf("expected BUILD_FROM_SOURCE to be absent on arm64, got %v", result)
	}
}

func TestResolveEnvWhen_MultipleArchAllowed(t *testing.T) {
	entries := map[string]EnvWhenEntry{
		"VAR": {
			Value: "x",
			Conditions: []EnvWhenCondition{
				{Type: "arch", Arch: []string{"amd64", "386"}},
			},
		},
	}
	if r := ResolveEnvWhen(entries, "amd64"); r["VAR"] != "x" {
		t.Error("expected VAR on amd64")
	}
	if r := ResolveEnvWhen(entries, "386"); r["VAR"] != "x" {
		t.Error("expected VAR on 386")
	}
	if r := ResolveEnvWhen(entries, "arm64"); r["VAR"] != "" {
		t.Error("expected VAR absent on arm64")
	}
}

func TestResolveEnvWhen_MultipleConditionsAnded(t *testing.T) {
	// Two arch conditions: both must pass (unusual but tests AND logic)
	entries := map[string]EnvWhenEntry{
		"VAR": {
			Value: "x",
			Conditions: []EnvWhenCondition{
				{Type: "arch", Arch: []string{"amd64"}},
				{Type: "arch", Arch: []string{"amd64", "arm64"}},
			},
		},
	}
	// amd64 satisfies both conditions
	if r := ResolveEnvWhen(entries, "amd64"); r["VAR"] != "x" {
		t.Error("expected VAR on amd64 when both conditions pass")
	}
	// arm64 satisfies second but not first
	if r := ResolveEnvWhen(entries, "arm64"); r["VAR"] != "" {
		t.Error("expected VAR absent on arm64 when first condition fails")
	}
}

func TestResolveEnvWhen_UnknownConditionType_Excluded(t *testing.T) {
	// Unknown condition types conservatively exclude the var
	entries := map[string]EnvWhenEntry{
		"VAR": {
			Value: "x",
			Conditions: []EnvWhenCondition{
				{Type: "unknown_future_type"},
			},
		},
	}
	result := ResolveEnvWhen(entries, "amd64")
	if _, ok := result["VAR"]; ok {
		t.Error("expected VAR excluded for unknown condition type")
	}
}
