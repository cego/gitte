package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"gopkg.in/yaml.v3"
)

// StartupCheck is the interface for all startup check types
type StartupCheck interface {
	GetType() string
	GetHint() string
	GetNeeds() []string
	Check(ctx context.Context, cwd string) error
}

// BaseStartupCheck holds common fields for all check types
type BaseStartupCheck struct {
	Type  string   `yaml:"type"`
	Hint  string   `yaml:"hint,omitempty"`
	Needs []string `yaml:"needs,omitempty"`
}

func (b *BaseStartupCheck) GetHint() string { return b.Hint }
func (b *BaseStartupCheck) GetType() string { return b.Type }
func (b *BaseStartupCheck) GetNeeds() []string {
	if b.Needs == nil {
		return []string{}
	}
	return b.Needs
}

// ShellStartupCheck runs a shell script
type ShellStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Shell            string `yaml:"shell"`
	Script           string `yaml:"script"`
}

func (s *ShellStartupCheck) Check(_ context.Context, cwd string) error {
	cmd := exec.Command(s.Shell, "-c", s.Script) //nolint:gosec
	cmd.Dir = cwd
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("shell script exited with code %d", exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// CommandStartupCheck runs a command and checks exit code
type CommandStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Command          []string `yaml:"cmd"`
}

func (s *CommandStartupCheck) Check(_ context.Context, cwd string) error {
	if len(s.Command) == 0 {
		return fmt.Errorf("command check has no command")
	}
	cmd := exec.Command(s.Command[0], s.Command[1:]...) //nolint:gosec
	cmd.Dir = cwd
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("command exited with code %d", exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// YamlPathPresentStartupCheck checks that a YAML path exists in a file
type YamlPathPresentStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Path             string `yaml:"path"`
	File             string `yaml:"file"`
}

func (s *YamlPathPresentStartupCheck) Check(_ context.Context, _ string) error {
	path, err := goyaml.PathString(s.Path)
	if err != nil {
		return fmt.Errorf("invalid yaml path: %w", err)
	}

	filePath := s.File
	if strings.HasPrefix(filePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to resolve home directory: %w", err)
		}
		filePath = strings.Replace(filePath, "~/", homeDir+string(os.PathSeparator), 1)
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	f, err := os.Open(absPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	_, err = path.ReadNode(f)
	return err
}

// StartupCheckMap is a map of startup checks with custom YAML unmarshaling
type StartupCheckMap map[string]StartupCheck

func (cm *StartupCheckMap) UnmarshalYAML(value *yaml.Node) error {
	*cm = make(StartupCheckMap)
	var rawMap map[string]yaml.Node
	if err := value.Decode(&rawMap); err != nil {
		return err
	}

	for k, v := range rawMap {
		var typeHelper struct {
			Type string `yaml:"type"`
		}
		if err := v.Decode(&typeHelper); err != nil {
			return err
		}

		var check StartupCheck
		switch typeHelper.Type {
		case "command":
			check = &CommandStartupCheck{}
		case "shell":
			check = &ShellStartupCheck{}
		case "yaml-path-present":
			check = &YamlPathPresentStartupCheck{}
		default:
			return fmt.Errorf("unknown check type: %s", typeHelper.Type)
		}

		if err := v.Decode(check); err != nil {
			return err
		}
		(*cm)[k] = check
	}

	return nil
}
