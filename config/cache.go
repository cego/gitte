package config

import (
	"context"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

type GitteCache struct {
	Cwd           string       `yaml:"cwd"`
	RemoteGitRepo string       `yaml:"remote_git_repo"`
	RemoteGitFile string       `yaml:"remote_git_file"`
	RemoteGitRef  string       `yaml:"remote_git_ref"`
	GitteConfig   *GitteConfig `yaml:"config"`
}

const cacheFileName = ".gitte-cache.yml"

func (c *GitteCache) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	cacheFilePath := filepath.Join(c.Cwd, cacheFileName)
	err = os.WriteFile(cacheFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func LoadCache(cwd string) (*GitteCache, error) {
	// if ConfigCache file does not exist, return nil
	cacheFilePath := filepath.Join(cwd, cacheFileName)
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		return &GitteCache{Cwd: cwd}, nil
	}
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return nil, err
	}

	var c GitteCache
	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

const cacheContextKey = "settingsCache"

func LoadCacheToContext(ctx context.Context, cwd string) (context.Context, error) {
	settingsCache, err := LoadCache(cwd)
	if err != nil {
		return nil, err
	}

	if settingsCache != nil {
		ctx = context.WithValue(ctx, cacheContextKey, settingsCache)
	}

	return ctx, nil
}

func CacheFromContext(ctx context.Context) *GitteCache {
	if sc, ok := ctx.Value(cacheContextKey).(*GitteCache); ok {
		return sc
	}

	return nil
}
