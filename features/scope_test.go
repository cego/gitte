package features

import (
	"testing"

	"github.com/cego/gitte/state"
)

func TestProjectMatchesOverrideScope_NilOverride(t *testing.T) {
	if ProjectMatchesOverrideScope("proj", "gitlab.cego.dk", "cego/monolith", nil) {
		t.Error("expected false for nil override")
	}
}

func TestProjectMatchesOverrideScope_ExplicitProjects(t *testing.T) {
	override := &state.ScopeOverride{
		Projects: []string{"monolith", "mysql"},
	}
	if !ProjectMatchesOverrideScope("monolith", "gitlab.cego.dk", "cego/monolith", override) {
		t.Error("expected monolith to match")
	}
	if ProjectMatchesOverrideScope("redis", "gitlab.cego.dk", "cego/redis", override) {
		t.Error("expected redis to not match")
	}
}

func TestProjectMatchesOverrideScope_GitlabGroup(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "cego"},
		},
	}
	if !ProjectMatchesOverrideScope("monolith", "gitlab.cego.dk", "cego/monolith", override) {
		t.Error("expected cego/monolith to match group cego")
	}
	if ProjectMatchesOverrideScope("promo", "gitlab.cego.dk", "spilnu/promo", override) {
		t.Error("expected spilnu/promo to not match group cego")
	}
}

func TestProjectMatchesOverrideScope_GitlabGroupDelimiter(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "spilnu"},
		},
	}
	if ProjectMatchesOverrideScope("foo", "gitlab.cego.dk", "spilnu-legacy/foo", override) {
		t.Error("expected spilnu-legacy/foo to not match group spilnu")
	}
	if !ProjectMatchesOverrideScope("promo", "gitlab.cego.dk", "spilnu/services/promo", override) {
		t.Error("expected spilnu/services/promo to match group spilnu")
	}
}

func TestProjectMatchesOverrideScope_ExcludeProjects(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "cego", ExcludeProjects: []string{"mysql"}},
		},
	}
	if !ProjectMatchesOverrideScope("monolith", "gitlab.cego.dk", "cego/monolith", override) {
		t.Error("expected monolith to match (not excluded)")
	}
	if ProjectMatchesOverrideScope("mysql", "gitlab.cego.dk", "cego/mysql", override) {
		t.Error("expected mysql to be excluded")
	}
}

func TestProjectMatchesOverrideScope_GithubOrg(t *testing.T) {
	override := &state.ScopeOverride{
		GithubOrgs: []state.ScopeOverrideOrg{
			{Host: "github.com", Org: "cego"},
		},
	}
	if !ProjectMatchesOverrideScope("gitte", "github.com", "cego/gitte", override) {
		t.Error("expected cego/gitte to match org cego")
	}
	if ProjectMatchesOverrideScope("other", "github.com", "other-org/repo", override) {
		t.Error("expected other-org/repo to not match org cego")
	}
}

func TestProjectMatchesOverrideScope_EmptyOverride(t *testing.T) {
	override := &state.ScopeOverride{}
	if ProjectMatchesOverrideScope("proj", "gitlab.cego.dk", "cego/proj", override) {
		t.Error("expected false for empty override")
	}
}

func TestBuildScopeTree_GroupsProjectsByHostAndNamespace(t *testing.T) {
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
		"promo":    {Host: "gitlab.cego.dk", Path: "spilnu/services/promo"},
	}
	rows := BuildScopeTree(projects)

	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	if rows[0].Kind != ScopeRowHost {
		t.Errorf("expected first row to be host, got %v", rows[0].Kind)
	}
	if rows[0].Label != "gitlab.cego.dk" {
		t.Errorf("expected label gitlab.cego.dk, got %q", rows[0].Label)
	}
}

func TestCheckedStateToOverride_AllChecked(t *testing.T) {
	checked := map[string]bool{"monolith": true, "mysql": true}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	override := CheckedStateToOverride(checked, projects)
	if override != nil {
		t.Error("expected nil override when all checked")
	}
}

func TestCheckedStateToOverride_NoneChecked(t *testing.T) {
	checked := map[string]bool{"monolith": false, "mysql": false}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	override := CheckedStateToOverride(checked, projects)
	if override == nil {
		t.Fatal("expected non-nil override when none checked")
	}
	if len(override.Projects) != 0 || len(override.GitlabGroups) != 0 || len(override.GithubOrgs) != 0 {
		t.Error("expected empty override")
	}
}

func TestCheckedStateToOverride_PartialGroup(t *testing.T) {
	checked := map[string]bool{"monolith": true, "mysql": false}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	override := CheckedStateToOverride(checked, projects)
	if override == nil {
		t.Fatal("expected non-nil override")
	}
	if len(override.GitlabGroups) != 1 {
		t.Fatalf("expected 1 gitlab group, got %d", len(override.GitlabGroups))
	}
	g := override.GitlabGroups[0]
	if g.Group != "cego" {
		t.Errorf("expected group cego, got %q", g.Group)
	}
	if len(g.ExcludeProjects) != 1 || g.ExcludeProjects[0] != "mysql" {
		t.Errorf("expected exclude [mysql], got %v", g.ExcludeProjects)
	}
}

func TestOverrideToCheckedState(t *testing.T) {
	override := &state.ScopeOverride{
		GitlabGroups: []state.ScopeOverrideGroup{
			{Host: "gitlab.cego.dk", Group: "cego", ExcludeProjects: []string{"mysql"}},
		},
	}
	projects := map[string]ScopeProject{
		"monolith": {Host: "gitlab.cego.dk", Path: "cego/monolith"},
		"mysql":    {Host: "gitlab.cego.dk", Path: "cego/mysql"},
	}
	checked := OverrideToCheckedState(override, projects)
	if !checked["monolith"] {
		t.Error("expected monolith checked")
	}
	if checked["mysql"] {
		t.Error("expected mysql unchecked")
	}
}
