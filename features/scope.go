package features

import (
	"strings"

	"github.com/cego/gitte/state"
)

// ProjectMatchesOverrideScope checks if a project is included in an override scope.
// projName is the config key, host and path come from config.ParseRemoteURL.
// Returns false if override is nil (caller should use config scope instead).
func ProjectMatchesOverrideScope(projName, host, path string, override *state.ScopeOverride) bool {
	if override == nil {
		return false
	}

	for _, p := range override.Projects {
		if p == projName {
			return true
		}
	}

	for _, gs := range override.GitlabGroups {
		if gs.Host != host {
			continue
		}
		if path == gs.Group || strings.HasPrefix(path, gs.Group+"/") {
			if !containsString(gs.ExcludeProjects, projName) {
				return true
			}
		}
	}

	for _, ghs := range override.GithubOrgs {
		if ghs.Host != host {
			continue
		}
		if strings.HasPrefix(path, ghs.Org+"/") {
			if !containsString(ghs.ExcludeProjects, projName) {
				return true
			}
		}
	}

	return false
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
