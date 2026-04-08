package config

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// DoctorCheck defines a config-driven diagnostic check whose output is captured
// and shown in the doctor report alongside a pass/fail status.
// Unlike startup checks, the command output is always displayed regardless of
// exit code.
type DoctorCheck struct {
	Type   string   `yaml:"type"`
	Cmd    []string `yaml:"cmd,omitempty"`
	Script string   `yaml:"script,omitempty"`
	Shell  string   `yaml:"shell,omitempty"`
	Hint   string   `yaml:"hint,omitempty"`
}

// Run executes the check and returns combined stdout+stderr output and whether
// the command exited successfully.
func (d *DoctorCheck) Run(ctx context.Context, cwd string) (output string, pass bool) {
	var cmd *exec.Cmd
	switch d.Type {
	case "shell":
		shell := d.Shell
		if shell == "" {
			shell = "sh"
		}
		cmd = exec.CommandContext(ctx, shell, "-c", d.Script) //nolint:gosec
	default: // "command"
		if len(d.Cmd) == 0 {
			return "no command configured", false
		}
		cmd = exec.CommandContext(ctx, d.Cmd[0], d.Cmd[1:]...) //nolint:gosec
	}
	cmd.Dir = cwd
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err == nil
}

// DoctorCheckMap is a map of named doctor checks with custom YAML unmarshaling.
type DoctorCheckMap map[string]DoctorCheck

func (m *DoctorCheckMap) UnmarshalYAML(value *yaml.Node) error {
	*m = make(DoctorCheckMap)
	var rawMap map[string]yaml.Node
	if err := value.Decode(&rawMap); err != nil {
		return err
	}
	for k, v := range rawMap {
		var check DoctorCheck
		if err := v.Decode(&check); err != nil {
			return fmt.Errorf("doctor check %q: %w", k, err)
		}
		if check.Type == "" {
			if check.Script != "" {
				check.Type = "shell"
			} else {
				check.Type = "command"
			}
		}
		(*m)[k] = check
	}
	return nil
}
