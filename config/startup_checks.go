package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	goyaml "github.com/goccy/go-yaml"
	"gopkg.in/yaml.v3"
)

// StartupCheck is the interface for all startup check types
type StartupCheck interface {
	GetType() string
	GetHint() string
	GetGuidance() *StartupGuidance
	GetNeeds() []string
	Check(ctx context.Context, cwd string, stdout, stderr io.Writer) error
}

// BaseStartupCheck holds common fields for all check types
type BaseStartupCheck struct {
	Type     string           `yaml:"type"`
	Hint     string           `yaml:"hint,omitempty"`
	Needs    []string         `yaml:"needs,omitempty"`
	Guidance *StartupGuidance `yaml:"guidance,omitempty"`
}

// StartupGuidance generates Markdown instructions after a failed check.
type StartupGuidance struct {
	Shell  string `yaml:"shell"`
	Script string `yaml:"script"`
}

func (b *BaseStartupCheck) GetGuidance() *StartupGuidance { return b.Guidance }
func (b *BaseStartupCheck) GetHint() string               { return b.Hint }
func (b *BaseStartupCheck) GetType() string               { return b.Type }
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

func (s *ShellStartupCheck) Check(ctx context.Context, cwd string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, s.Shell, "-c", s.Script) //nolint:gosec
	cmd.Dir = cwd
	return runStartupCommand(ctx, cmd, "shell script", stdout, stderr)
}

// CommandStartupCheck runs a command and checks exit code
type CommandStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Command          []string `yaml:"cmd"`
}

func (s *CommandStartupCheck) Check(ctx context.Context, cwd string, stdout, stderr io.Writer) error {
	if len(s.Command) == 0 {
		return fmt.Errorf("command check has no command")
	}
	cmd := exec.CommandContext(ctx, s.Command[0], s.Command[1:]...) //nolint:gosec
	cmd.Dir = cwd
	return runStartupCommand(ctx, cmd, "command", stdout, stderr)
}

// runStartupCommand retains bounded diagnostics while forwarding output to the caller.
func runStartupCommand(ctx context.Context, cmd *exec.Cmd, label string, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	out, errOut := &diagnosticBuffer{}, &diagnosticBuffer{}
	cmd.Stdout = io.MultiWriter(stdout, out)
	cmd.Stderr = io.MultiWriter(stderr, errOut)
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", label, ctx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail := errOut.String()
			if detail == "" {
				detail = out.String()
			}
			if detail != "" {
				return fmt.Errorf("%s exited with code %d: %s", label, exitErr.ExitCode(), detail)
			}
			return fmt.Errorf("%s exited with code %d", label, exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// diagnosticBuffer retains the final 16 KiB without blocking command output.
type diagnosticBuffer struct {
	data      []byte
	truncated bool
}

func (b *diagnosticBuffer) Write(p []byte) (int, error) {
	const limit = 16 * 1024
	n := len(p)
	if len(b.data)+n > limit {
		b.truncated = true
		if n >= limit {
			b.data = append(b.data[:0], p[n-limit:]...)
			return n, nil
		}
		b.data = b.data[len(b.data)+n-limit:]
	}
	b.data = append(b.data, p...)
	return n, nil
}

func (b *diagnosticBuffer) String() string {
	text := strings.TrimSpace(string(b.data))
	if b.truncated {
		return "[earlier output omitted]\n" + text
	}
	return text
}

// YamlPathPresentStartupCheck checks that a YAML path exists in a file
type YamlPathPresentStartupCheck struct {
	BaseStartupCheck `yaml:",inline"`
	Path             string `yaml:"path"`
	File             string `yaml:"file"`
}

func (s *YamlPathPresentStartupCheck) Check(_ context.Context, _ string, _, _ io.Writer) error {
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

	_, readErr := path.ReadNode(f)
	if closeErr := f.Close(); closeErr != nil && readErr == nil {
		return closeErr
	}
	return readErr
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
