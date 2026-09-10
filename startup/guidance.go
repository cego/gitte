package startup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cego/gitte/config"
)

const guidanceTimeout = 3 * time.Second
const guidanceOutputLimit = 16 * 1024

// checkFailure keeps presentation separate from the check's diagnostic error.
type checkFailure struct {
	cause    error
	hint     string
	markdown bool
}

func (f *checkFailure) Error() string { return f.cause.Error() }
func (f *checkFailure) Unwrap() error { return f.cause }

func describeFailure(ctx context.Context, check config.StartupCheck, name, cwd string, cause error) error {
	failure := &checkFailure{cause: cause, hint: check.GetHint()}
	if guidance := check.GetGuidance(); guidance != nil && ctx.Err() == nil {
		if text, err := generateGuidance(ctx, guidance, name, cwd, guidanceTimeout); err == nil {
			failure.hint, failure.markdown = text, true
		}
	}
	return failure
}

func generateGuidance(ctx context.Context, guidance *config.StartupGuidance, name, cwd string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(guidance.Shell) == "" || strings.TrimSpace(guidance.Script) == "" {
		return "", fmt.Errorf("guidance requires shell and script")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, guidance.Shell, "-c", guidance.Script) //nolint:gosec
	cmd.Dir = cwd
	cmd.Env = append(cmd.Environ(), "GITTE_CHECK_NAME="+name)
	cmd.WaitDelay = 100 * time.Millisecond
	cleanup := prepareGuidanceProcess(cmd)
	defer cleanup()
	output := &guidanceBuffer{cancel: cancel}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("generating guidance: %w", err)
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if output.overflow {
		return "", fmt.Errorf("guidance exceeds %d bytes", guidanceOutputLimit)
	}
	text := strings.TrimSpace(string(output.data))
	if text == "" {
		return "", fmt.Errorf("guidance is empty")
	}
	return text, nil
}

type guidanceBuffer struct {
	data     []byte
	overflow bool
	cancel   context.CancelFunc
}

func (b *guidanceBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > guidanceOutputLimit {
		b.overflow = true
		b.cancel()
		return len(p), nil
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
