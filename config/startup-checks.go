package config

import (
	"context"
	"fmt"
	"gitte/executor"

	"go.yaml.in/yaml/v3"
)

type StartupCheck interface {
	GetType() string
	GetHint() string
	GetNeeds() []string
	Check(ctx context.Context, cwd string) error
}

type BaseStartupCheck struct {
	Type  string   `yaml:"type"`
	Hint  string   `yaml:"hint,omitempty"`
	Needs []string `yaml:"needs,omitempty"`
}

func (b *BaseStartupCheck) GetHint() string {
	return b.Hint
}

func (b *BaseStartupCheck) GetType() string {
	return b.Type
}

func (b *BaseStartupCheck) GetNeeds() []string {
	return []string{}
}

type ShellStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Shell            string `yaml:"shell"`
	Script           string `yaml:"script"`
}

func (s *ShellStartupCheck) Check(ctx context.Context, cwd string) error {
	res, err := executor.ExecuteSyncInDir(cwd, s.Shell, "-c", s.Script)

	if err != nil {
		return err
	}

	if res.ExitCode != 0 {
		return fmt.Errorf("shell script exited with code %d", res.ExitCode)
	}

	return nil
}

type CommandStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Command          []string `yaml:"cmd"`
}

func (s *CommandStartupCheck) Check(ctx context.Context, cwd string) error {
	res, err := executor.ExecuteSyncInDir(cwd, s.Command[0], s.Command[1:]...)

	if err != nil {
		return err
	}

	if res.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", res.ExitCode)
	}

	return nil
}

type YamlPathPresentStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Path             string `yaml:"path"`
	File             string `yaml:"file"`
}

func (s *YamlPathPresentStartupCheck) Check(ctx context.Context, cwd string) error {
	return nil
}

type StartupCheckMap map[string]StartupCheck

func (cm *StartupCheckMap) UnmarshalYAML(value *yaml.Node) error {
	// Initialize the map
	*cm = make(StartupCheckMap)
	var rawMap map[string]yaml.Node
	if err := value.Decode(&rawMap); err != nil {
		return err
	}

	for k, v := range rawMap {
		// Create a temporary struct just to extract the "type" field
		var typeHelper struct {
			Type string `yaml:"type"`
		}

		// Decode only the type field
		if err := v.Decode(&typeHelper); err != nil {
			return err
		}

		var check StartupCheck
		// Switch based on the type to create the correct struct
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

		// Now decode the full node into the concrete struct
		if err := v.Decode(check); err != nil {
			return err
		}

		// Add it to our map
		(*cm)[k] = check
	}

	return nil
}
