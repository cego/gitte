package config

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"gitte/executor"
	"io"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"
)

type GitteContext struct {
	config *GitteConfig
	cwd    string
}

const ConfigPath = ".gitte.yml"
const DotEnvPath = ".gitte-env"
const OverridePath = ".gitte.override.yml" // TODO
const CacheDir = ".gitte"                  // TODO maybe make migration from .gitte-ConfigCache.json

func LoadConfig(ctx context.Context, fd FileDefinition) (*GitteConfig, error) {
	if fd.IsEnv {
		return LoadCachedGitteConfigFromEnvAndAsyncRefresh(ctx, fd)
	}

	// Load YAML content into GitteConfig struct
	return LoadGitteConfigFromYAML(fd.ConfigContent)
}

func LoadCachedGitteConfigFromEnvAndAsyncRefresh(ctx context.Context, fd FileDefinition) (*GitteConfig, error) {
	dotenv, err := godotenv.Parse(bytes.NewReader(fd.ConfigContent))
	if err != nil {
		return nil, err
	}

	// Assert required fields
	remoteGitRepo, ok := dotenv["REMOTE_GIT_REPO"]
	if !ok || remoteGitRepo == "" {
		return nil, errors.New("REMOTE_GIT_REPO isn't defined in .gitte.env")
	}
	remoteGitFile, ok := dotenv["REMOTE_GIT_FILE"]
	if !ok || remoteGitFile == "" {
		return nil, errors.New("REMOTE_GIT_FILE isn't defined in .gitte.env")
	}
	remoteGitRef, ok := dotenv["REMOTE_GIT_REF"]
	if !ok || remoteGitRef == "" {
		return nil, errors.New("REMOTE_GIT_REF isn't defined in .gitte.env")
	}

	settingsCache := CacheFromContext(ctx)

	if settingsCache.GitteConfig == nil || settingsCache.RemoteGitFile != remoteGitFile || settingsCache.RemoteGitRef != remoteGitRef || settingsCache.RemoteGitRepo != remoteGitRepo {
		loadedConfig, err := LoadGitteConfigFromEnv(fd, remoteGitRepo, remoteGitRef, remoteGitFile)
		if err != nil {
			return nil, err
		}
		settingsCache.RemoteGitFile = remoteGitFile
		settingsCache.RemoteGitRef = remoteGitRef
		settingsCache.RemoteGitRepo = remoteGitRepo
		settingsCache.GitteConfig = loadedConfig
		if settingsCache.Save() != nil {
			fmt.Println("Error saving refreshed gitte config to cache:", err)
		}
		return loadedConfig, nil
	}

	go func() {
		loadedConfig, err := LoadGitteConfigFromEnv(fd, remoteGitRepo, remoteGitRef, remoteGitFile)

		if err != nil {
			fmt.Println("Error refreshing remote gitte config:", err)
			return
		}

		// TODO mutex?
		settingsCache.RemoteGitFile = remoteGitFile
		settingsCache.RemoteGitRef = remoteGitRef
		settingsCache.RemoteGitRepo = remoteGitRepo
		settingsCache.GitteConfig = loadedConfig
		if settingsCache.Save() != nil {
			fmt.Println("Error saving refreshed gitte config to cache:", err)
		}
	}()

	return settingsCache.GitteConfig, nil
}

func LoadGitteConfigFromEnv(fd FileDefinition, remoteGitRepo string, remoteGitRef string, remoteGitFile string) (*GitteConfig, error) {
	res, err := executor.ExecuteSync("git", "archive", "--remote="+remoteGitRepo, remoteGitRef, remoteGitFile)

	if err != nil {
		return nil, fmt.Errorf("failed to execute git archive command: %w", err)
	}

	// Untar the result to extract the file content
	tarReader := tar.NewReader(bytes.NewReader(res.Stdout))
	var fileContent []byte

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar archive: %w", err)
		}

		// Extract the file matching remoteGitFile
		if header.Name == remoteGitFile {
			fileContent, err = io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read file from tar archive: %w", err)
			}
			break
		}
	}

	if fileContent == nil {
		return nil, fmt.Errorf("file %s not found in tar archive", remoteGitFile)
	}
	return LoadGitteConfigFromYAML(fileContent)
}

type FileDefinition struct {
	ConfigContent []byte
	IsEnv         bool
	Directory     string
}

var (
	ErrGitteConfigNotFound = errors.New("Gitte configuration file not found")
)

func ResolveGitteDir(cwd string) (FileDefinition, error) {
	return resolveGitteDirWithPath(cwd)
}

func resolveGitteDirWithPath(cwd string) (FileDefinition, error) {
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return FileDefinition{}, err
	}

	if f, err := os.OpenFile(filepath.Join(cwd, DotEnvPath), os.O_RDONLY, 0644); err == nil {
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			return FileDefinition{}, err
		}
		return FileDefinition{ConfigContent: content, IsEnv: true, Directory: cwd}, nil
	}

	if f, err := os.OpenFile(filepath.Join(cwd, ConfigPath), os.O_RDONLY, 0644); err == nil {
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			return FileDefinition{}, err
		}

		return FileDefinition{ConfigContent: content, IsEnv: false, Directory: cwd}, nil
	}

	parentDir := filepath.Join(cwd, "..")
	if parentDir == cwd {
		return FileDefinition{}, ErrGitteConfigNotFound
	}

	return resolveGitteDirWithPath(parentDir)
}

func LoadGitteConfigFromYAML(content []byte) (*GitteConfig, error) {
	var config GitteConfig
	err := yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
