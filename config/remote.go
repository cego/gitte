package config

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// RemoteConfigCache holds cached remote config state
type RemoteConfigCache struct {
	RemoteGitRepo string    `yaml:"remote_git_repo"`
	RemoteGitFile string    `yaml:"remote_git_file"`
	RemoteGitRef  string    `yaml:"remote_git_ref"`
	FetchedAt     time.Time `yaml:"fetched_at"`
	Content       string    `yaml:"content"`
}

var remoteMu sync.Mutex

// LoadRemoteConfig fetches remote config via git archive and caches it
// If a valid cache entry exists, it returns the cached config and refreshes async
func LoadRemoteConfig(ctx context.Context, envContent []byte, cache *RemoteConfigCache) (*GitteConfig, *RemoteConfigCache, error) {
	dotenv, err := godotenv.Parse(bytes.NewReader(envContent))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse .gitte-env: %w", err)
	}

	repo, ok := dotenv["REMOTE_GIT_REPO"]
	if !ok || repo == "" {
		return nil, nil, errors.New("REMOTE_GIT_REPO isn't defined in .gitte-env")
	}
	file, ok := dotenv["REMOTE_GIT_FILE"]
	if !ok || file == "" {
		return nil, nil, errors.New("REMOTE_GIT_FILE isn't defined in .gitte-env")
	}
	ref, ok := dotenv["REMOTE_GIT_REF"]
	if !ok || ref == "" {
		return nil, nil, errors.New("REMOTE_GIT_REF isn't defined in .gitte-env")
	}

	cacheValid := cache != nil &&
		cache.RemoteGitRepo == repo &&
		cache.RemoteGitFile == file &&
		cache.RemoteGitRef == ref &&
		cache.Content != ""

	if !cacheValid {
		// Synchronous fetch
		newCache, err := fetchRemoteConfig(repo, ref, file)
		if err != nil {
			return nil, nil, err
		}
		cfg, err := LoadGitteConfigFromYAML([]byte(newCache.Content))
		if err != nil {
			return nil, nil, err
		}
		return cfg, newCache, nil
	}

	// Use cache, refresh async
	cfg, err := LoadGitteConfigFromYAML([]byte(cache.Content))
	if err != nil {
		return nil, nil, err
	}

	go func() {
		newCache, err := fetchRemoteConfig(repo, ref, file)
		if err != nil {
			fmt.Printf("warning: async remote config refresh failed: %v\n", err)
			return
		}
		remoteMu.Lock()
		*cache = *newCache
		remoteMu.Unlock()
	}()

	return cfg, cache, nil
}

func fetchRemoteConfig(repo, ref, file string) (*RemoteConfigCache, error) {
	cmd := exec.Command("git", "archive", "--remote="+repo, ref, file) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git archive failed: %w\nstderr: %s", err, stderr.String())
	}

	tarReader := tar.NewReader(&stdout)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}
		if header.Name == file {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read file from tar: %w", err)
			}
			return &RemoteConfigCache{
				RemoteGitRepo: repo,
				RemoteGitFile: file,
				RemoteGitRef:  ref,
				FetchedAt:     time.Now(),
				Content:       string(content),
			}, nil
		}
	}
	return nil, fmt.Errorf("file %q not found in git archive", file)
}

// MarshalYAMLRemoteCache serializes cache to YAML bytes
func MarshalYAMLRemoteCache(cache *RemoteConfigCache) ([]byte, error) {
	return yaml.Marshal(cache)
}
