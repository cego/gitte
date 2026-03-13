package state

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const StateFileName = ".gitte-state.yml"
const StateVersion = 1

// GitteState holds all local machine state
type GitteState struct {
	Version  int                     `yaml:"version"`
	Toggles  map[string]bool         `yaml:"toggles,omitempty"`
	Features map[string]FeatureState `yaml:"features,omitempty"`
	Cache    StateCache              `yaml:"cache,omitempty"`
}

// FeatureState holds the enabled status and optional scope override for a feature gate
type FeatureState struct {
	Enabled       bool           `yaml:"enabled"`
	OverrideScope *ScopeOverride `yaml:"override_scope,omitempty"`
}

// ScopeOverride narrows a feature gate's configured scope on this machine
type ScopeOverride struct {
	Projects     []string             `yaml:"projects,omitempty"`
	GitlabGroups []ScopeOverrideGroup `yaml:"gitlab_groups,omitempty"`
	GithubOrgs   []ScopeOverrideOrg   `yaml:"github_orgs,omitempty"`
}

// ScopeOverrideGroup scopes a feature gate to a GitLab group with optional exclusions
type ScopeOverrideGroup struct {
	Host            string   `yaml:"host"`
	Group           string   `yaml:"group"`
	ExcludeProjects []string `yaml:"exclude_projects,omitempty"`
}

// ScopeOverrideOrg scopes a feature gate to a GitHub org with optional exclusions
type ScopeOverrideOrg struct {
	Host            string   `yaml:"host"`
	Org             string   `yaml:"org"`
	ExcludeProjects []string `yaml:"exclude_projects,omitempty"`
}

// StateCache holds cached remote config metadata
type StateCache struct {
	RemoteConfig *RemoteConfigCacheEntry `yaml:"remote_config,omitempty"`
}

// RemoteConfigCacheEntry holds cached remote config
type RemoteConfigCacheEntry struct {
	RemoteGitRepo string    `yaml:"remote_git_repo"`
	RemoteGitRef  string    `yaml:"remote_git_ref"`
	RemoteGitFile string    `yaml:"remote_git_file"`
	FetchedAt     time.Time `yaml:"fetched_at"`
	Content       string    `yaml:"content"`
}

// Load loads the state file from the given directory, returning a default state if not found
func Load(dir string) (*GitteState, error) {
	path := filepath.Join(dir, StateFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &GitteState{
			Version:  StateVersion,
			Toggles:  make(map[string]bool),
			Features: make(map[string]FeatureState),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var s GitteState
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	if s.Toggles == nil {
		s.Toggles = make(map[string]bool)
	}
	if s.Features == nil {
		s.Features = make(map[string]FeatureState)
	}

	return &s, nil
}

// Save writes the state file to the given directory
func Save(dir string, s *GitteState) error {
	s.Version = StateVersion
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, StateFileName), data, 0644) //nolint:gosec
}

// EnsureGitignored adds .gitte-state.yml to .gitignore if not already present
func EnsureGitignored(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(data)
	if containsLine(content, StateFileName) {
		return nil
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) //nolint:gosec
	if err != nil {
		return err
	}

	_, err = f.WriteString("\n" + StateFileName + "\n")
	if closeErr := f.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}

func containsLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if l == line {
			return true
		}
	}
	return false
}
