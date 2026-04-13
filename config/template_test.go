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

func TestResolveTemplates_TemplateExtendsTemplate(t *testing.T) {
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"base": {
				Vars: map[string]string{"stack": "base-{{.project}}"},
				Actions: map[string]ProjectAction{
					"up": {Groups: map[string][]string{
						"sn": {"deploy", "{{.stack}}"},
					}},
				},
			},
			"child": {
				Extends: []string{"base"},
				Actions: map[string]ProjectAction{
					"up": {Needs: []string{"mysql"}},
				},
			},
		},
		Projects: map[string]ProjectConfig{
			"myservice": {Remote: "git@example.com/x.git", Extends: "child"},
		},
	}

	if err := ResolveTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	up := cfg.Projects["myservice"].Actions["up"]
	if len(up.Needs) != 1 || up.Needs[0] != "mysql" {
		t.Errorf("expected needs=[mysql], got %v", up.Needs)
	}
	cmds := up.Groups["sn"]
	if len(cmds) < 2 || cmds[1] != "base-myservice" {
		t.Errorf("expected 'base-myservice', got %v", cmds)
	}
}

func TestResolveTemplates_MultipleParents(t *testing.T) {
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"sn-project": {
				Vars: map[string]string{"sn_stack": "spilnu-{{.project}}"},
				Actions: map[string]ProjectAction{
					"up": {Groups: map[string][]string{"sn": {"deploy", "{{.sn_stack}}"}}},
				},
			},
			"ht-project": {
				Vars: map[string]string{"ht_stack": "happytiger-{{.project}}"},
				Actions: map[string]ProjectAction{
					"up": {Groups: map[string][]string{"ht": {"deploy", "{{.ht_stack}}"}}},
				},
			},
			"site-project": {
				Extends: []string{"sn-project", "ht-project"},
			},
		},
		Projects: map[string]ProjectConfig{
			"myservice": {Remote: "git@example.com/x.git", Extends: "site-project"},
		},
	}

	if err := ResolveTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	up := cfg.Projects["myservice"].Actions["up"]
	if _, ok := up.Groups["sn"]; !ok {
		t.Error("expected sn group from sn-project parent")
	}
	if _, ok := up.Groups["ht"]; !ok {
		t.Error("expected ht group from ht-project parent")
	}
	if up.Groups["sn"][1] != "spilnu-myservice" {
		t.Errorf("expected 'spilnu-myservice', got %v", up.Groups["sn"])
	}
	if up.Groups["ht"][1] != "happytiger-myservice" {
		t.Errorf("expected 'happytiger-myservice', got %v", up.Groups["ht"])
	}
}

func TestResolveTemplates_TemplateCycle(t *testing.T) {
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"a": {Extends: []string{"b"}},
			"b": {Extends: []string{"a"}},
		},
		Projects: map[string]ProjectConfig{},
	}

	if err := ResolveTemplates(cfg); err == nil {
		t.Error("expected error for template cycle, got nil")
	}
}

func TestResolveTemplates_NeedsNilDoesNotOverride(t *testing.T) {
	// Project with no needs specified should inherit template needs
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"base": {Actions: map[string]ProjectAction{
				"up": {Needs: []string{"dep"}, Groups: map[string][]string{"sn": {"cmd"}}},
			}},
		},
		Projects: map[string]ProjectConfig{
			"myservice": {Remote: "git@example.com/x.git", Extends: "base",
				Actions: map[string]ProjectAction{
					"up": {Groups: map[string][]string{"ht": {"cmd2"}}}, // no Needs field
				},
			},
		},
	}

	if err := ResolveTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	up := cfg.Projects["myservice"].Actions["up"]
	if len(up.Needs) != 1 || up.Needs[0] != "dep" {
		t.Errorf("expected inherited needs=[dep], got %v", up.Needs)
	}
}

func TestResolveTemplates_EmptyNeedsClearsInherited(t *testing.T) {
	// Project with explicit needs: [] should clear template needs
	cfg := &GitteConfig{
		Templates: map[string]Template{
			"base": {Actions: map[string]ProjectAction{
				"up": {Needs: []string{"dep"}, Groups: map[string][]string{"sn": {"cmd"}}},
			}},
		},
		Projects: map[string]ProjectConfig{
			"myservice": {Remote: "git@example.com/x.git", Extends: "base",
				Actions: map[string]ProjectAction{
					"up": {Needs: []string{}}, // explicit empty
				},
			},
		},
	}

	if err := ResolveTemplates(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	up := cfg.Projects["myservice"].Actions["up"]
	if len(up.Needs) != 0 {
		t.Errorf("expected empty needs after explicit override, got %v", up.Needs)
	}
}
