package config

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"gitte/executor"
	"io"
	"io/fs"
	"os"

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
const CacheDir = ".gitte"                  // TODO maybe make migration from .gitte-cache.json

func LoadConfig(fd FileDefinition) (*GitteConfig, error) {
	if fd.IsEnv {
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

		fmt.Println("Executing command to fetch remote gitte config:", "git", "archive", "--remote="+remoteGitRepo, remoteGitRef, remoteGitFile)

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

	// Load YAML content into GitteConfig struct
	return LoadGitteConfigFromYAML(fd.ConfigContent)
}

type FileDefinition struct {
	ConfigContent []byte
	IsEnv         bool
}

var (
	ErrGitteConfigNotFound = errors.New("Gitte configuration file not found")
)

func ResolveGitteDir(fs fs.FS) (FileDefinition, error) {
	if f, err := fs.Open(DotEnvPath); err == nil {
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			return FileDefinition{}, err
		}
		return FileDefinition{ConfigContent: content, IsEnv: true}, nil
	}

	if f, err := fs.Open(ConfigPath); err == nil {
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			return FileDefinition{}, err
		}
		return FileDefinition{ConfigContent: content, IsEnv: false}, nil
	}

	parentDir := os.DirFS("..")
	if parentDir != fs {
		return ResolveGitteDir(parentDir)
	}

	return FileDefinition{}, ErrGitteConfigNotFound
}

func LoadGitteConfigFromYAML(content []byte) (*GitteConfig, error) {
	var config GitteConfig
	err := yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
