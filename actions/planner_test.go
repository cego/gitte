package actions

import (
	"gitte/config"
	"testing"
)

func testConfig() *config.GitteConfig {
	return &config.GitteConfig{
		Projects: map[string]config.ProjectConfig{
			"database": {
				Remote: "git@github.com:example/database.git",
				Actions: map[string]config.ProjectAction{
					"up": {
						Groups: map[string][]string{
							"prod":    {"echo", "database up prod"},
							"staging": {"echo", "database up staging"},
						},
					},
				},
			},
			"backend": {
				Remote: "git@github.com:example/backend.git",
				Actions: map[string]config.ProjectAction{
					"up": {
						Needs: []string{"database"},
						Groups: map[string][]string{
							"prod": {"echo", "backend up prod"},
						},
					},
					"build": {
						Groups: map[string][]string{
							"prod": {"echo", "backend build prod"},
						},
					},
				},
			},
		},
	}
}

func TestPlanActions_AllProjects(t *testing.T) {
	cfg := testConfig()
	keys := PlanActions(cfg, "up", "*", "*", false)

	if len(keys) == 0 {
		t.Fatal("expected keys, got none")
	}

	// Should have database:up:prod, database:up:staging, backend:up:prod
	found := map[string]bool{}
	for _, k := range keys {
		found[k.Project+":"+k.Action+":"+k.Group] = true
	}

	if !found["database:up:prod"] {
		t.Error("expected database:up:prod")
	}
	if !found["backend:up:prod"] {
		t.Error("expected backend:up:prod")
	}
}

func TestPlanActions_SpecificProject(t *testing.T) {
	cfg := testConfig()
	keys := PlanActions(cfg, "up", "database", "*", false)

	if len(keys) == 0 {
		t.Fatal("expected keys, got none")
	}

	for _, k := range keys {
		if k.Project != "database" {
			t.Errorf("unexpected project %q", k.Project)
		}
	}
}

func TestPlanActions_SpecificGroup(t *testing.T) {
	cfg := testConfig()
	keys := PlanActions(cfg, "up", "*", "prod", false)

	for _, k := range keys {
		if k.Group != "prod" {
			t.Errorf("expected group prod, got %q", k.Group)
		}
	}
}

func TestPlanActions_WithNeeds(t *testing.T) {
	cfg := testConfig()
	keys := PlanActions(cfg, "up", "backend", "*", true)

	// Should include backend and its dependency database
	found := map[string]bool{}
	for _, k := range keys {
		found[k.Project] = true
	}

	if !found["backend"] {
		t.Error("expected backend in results")
	}
	if !found["database"] {
		t.Error("expected database in results (as dependency)")
	}
}

func TestPlanActions_MultipleActions(t *testing.T) {
	cfg := testConfig()
	keys := PlanActions(cfg, "up+build", "backend", "*", false)

	found := map[string]bool{}
	for _, k := range keys {
		found[k.Project+":"+k.Action] = true
	}

	if !found["backend:up"] {
		t.Error("expected backend:up")
	}
	if !found["backend:build"] {
		t.Error("expected backend:build")
	}
}

func TestParseGitteString(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"up", []string{"up"}},
		{"up+build", []string{"up", "build"}},
		{"*", []string{"*"}},
		{"all", []string{"*"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := parseGitteString(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseGitteString(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseGitteString(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
