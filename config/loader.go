package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigPath = ".gitte.yml"
const OverridePath = ".gitte-override.yml"
const DotEnvPath = ".gitte-env"

var ErrGitteConfigNotFound = errors.New("gitte configuration file not found")

// FileDefinition holds raw config file content and metadata
type FileDefinition struct {
	ConfigContent []byte
	IsEnv         bool
	Directory     string
}

// ResolveGitteDir walks up from cwd to find .gitte.yml or .gitte-env
func ResolveGitteDir(cwd string) (FileDefinition, error) {
	absDir, err := filepath.Abs(cwd)
	if err != nil {
		return FileDefinition{}, err
	}
	return resolveGitteDirFrom(absDir)
}

func resolveGitteDirFrom(dir string) (FileDefinition, error) {
	// Check for .gitte-env first (remote config)
	if fd, err := tryReadFile(filepath.Join(dir, DotEnvPath)); err != nil {
		return FileDefinition{}, err
	} else if fd != nil {
		return FileDefinition{ConfigContent: fd, IsEnv: true, Directory: dir}, nil
	}

	// Check for .gitte.yml
	if fd, err := tryReadFile(filepath.Join(dir, ConfigPath)); err != nil {
		return FileDefinition{}, err
	} else if fd != nil {
		return FileDefinition{ConfigContent: fd, IsEnv: false, Directory: dir}, nil
	}

	parent := filepath.Dir(dir)
	if parent == dir {
		return FileDefinition{}, ErrGitteConfigNotFound
	}
	return resolveGitteDirFrom(parent)
}

// tryReadFile reads a file and returns its contents.
// Returns (nil, nil) if the file does not exist, (nil, err) on other errors.
func tryReadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	content, readErr := io.ReadAll(f)
	if closeErr := f.Close(); closeErr != nil && readErr == nil {
		return nil, closeErr
	}
	if readErr != nil {
		return nil, readErr
	}
	return content, nil
}

// LoadGitteConfigFromYAML parses raw YAML bytes into a GitteConfig
func LoadGitteConfigFromYAML(content []byte) (*GitteConfig, error) {
	var cfg GitteConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// MergeOverride merges an override config on top of the base config
// Override values win over base values (shallow at top level, deep for projects/actions).
func MergeOverride(base, override *GitteConfig) *GitteConfig {
	if override == nil {
		return base
	}

	// Merge projects: override wins for matching keys, base keeps others
	if override.Projects != nil {
		if base.Projects == nil {
			base.Projects = make(map[string]ProjectConfig)
		}
		for k, v := range override.Projects {
			base.Projects[k] = v
		}
	}

	// Merge action overrides
	if override.ActionOverride != nil {
		if base.ActionOverride == nil {
			base.ActionOverride = make(map[string]ActionOverride)
		}
		for k, v := range override.ActionOverride {
			base.ActionOverride[k] = v
		}
	}

	// Override searchFor if present
	if override.SearchFor != nil {
		base.SearchFor = override.SearchFor
	}

	return base
}

// LoadAndMergeConfig loads .gitte.yml and merges .gitte-override.yml if present
func LoadAndMergeConfig(fd FileDefinition) (*GitteConfig, error) {
	cfg, err := LoadGitteConfigFromYAML(fd.ConfigContent)
	if err != nil {
		return nil, err
	}

	// Try to load override
	overridePath := filepath.Join(fd.Directory, OverridePath)
	if overrideData, err := os.ReadFile(overridePath); err == nil {
		overrideCfg, err := LoadGitteConfigFromYAML(overrideData)
		if err != nil {
			return nil, err
		}
		cfg = MergeOverride(cfg, overrideCfg)
	}

	return cfg, nil
}
