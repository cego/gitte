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
