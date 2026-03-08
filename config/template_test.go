package config

import (
	"testing"
)

func TestResolveTemplates_Basic(t *testing.T) {
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"base": {
				Vars: map[string]string{
					"stack": "default-{{.project}}",
				},
				Actions: map[string]ProjectAction{
					"up": {
						Groups: map[string][]string{
							"prod": {"echo", "deploy {{.stack}}"},
						},
					},
				},
			},
		},
		Projects: map[string]ProjectConfig{
			"myservice": {
				Remote:  "git@github.com:example/myservice.git",
				Extends: "base",
			},
		},
	}

	if err := ResolveTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Templates should be cleared
	if cfg.Templates != nil {
		t.Error("expected templates to be cleared after resolution")
	}

	// Project should have the action
	proj := cfg.Projects["myservice"]
	if proj.Extends != "" {
		t.Error("expected extends to be cleared")
	}
	if proj.Actions == nil {
		t.Fatal("expected actions to be set")
	}
	upAction, ok := proj.Actions["up"]
	if !ok {
		t.Fatal("expected up action to exist")
	}
	if _, ok := upAction.Groups["prod"]; !ok {
		t.Fatal("expected prod group to exist")
	}

	// Verify variable substitution
	cmds := upAction.Groups["prod"]
	if len(cmds) < 2 || cmds[1] != "deploy default-myservice" {
		t.Errorf("expected 'deploy default-myservice', got %v", cmds)
	}
}

func TestResolveTemplates_ProjectVarsOverride(t *testing.T) {
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"base": {
				Vars: map[string]string{
					"stack": "default",
				},
				Actions: map[string]ProjectAction{
					"up": {
						Groups: map[string][]string{
							"prod": {"echo", "{{.stack}}"},
						},
					},
				},
			},
		},
		Projects: map[string]ProjectConfig{
			"myservice": {
				Remote:  "git@github.com:example/myservice.git",
				Extends: "base",
				Vars:    map[string]string{"stack": "custom-stack"},
			},
		},
	}

	if err := ResolveTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmds := cfg.Projects["myservice"].Actions["up"].Groups["prod"]
	if len(cmds) < 2 || cmds[1] != "custom-stack" {
		t.Errorf("expected 'custom-stack', got %v", cmds)
	}
}

func TestResolveTemplates_UnknownTemplate(t *testing.T) {
	cfg := &GitteConfig{
		Projects: map[string]ProjectConfig{
			"myservice": {
				Remote:  "git@github.com:example/myservice.git",
				Extends: "nonexistent",
			},
		},
	}

	if err := ResolveTemplates(cfg); err == nil {
		t.Error("expected error for unknown template, got nil")
	}
}
