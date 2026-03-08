package executor

import (
	"context"
	"time"
)

// StreamType indicates whether output came from stdout or stderr
type StreamType string

const (
	StderrStream StreamType = "stderr"
	StdoutStream StreamType = "stdout"
)

// Output holds a single line of output from a running task
type Output struct {
	Output  []byte
	CmdName string
	Stream  StreamType
}

// OutputHandler receives streaming output from task execution
type OutputHandler interface {
	HandleOutput(ctx context.Context, output Output) error
}

// NoopOutputHandler discards all output
type NoopOutputHandler struct{}

func (n NoopOutputHandler) HandleOutput(_ context.Context, _ Output) error { return nil }

// LogOutputHandler prints output to stdout with a prefix
type LogOutputHandler struct{}

func (l LogOutputHandler) HandleOutput(_ context.Context, output Output) error {
	return nil // replaced by plain/TUI writer
}

// ToChannelOutputHandler feeds output into a channel
type ToChannelOutputHandler struct {
	OutputCh chan<- Output
}

func (h ToChannelOutputHandler) HandleOutput(_ context.Context, output Output) error {
	select {
	case h.OutputCh <- output:
	default:
		// channel full, drop line (shouldn't happen with adequate buffer)
	}
	return nil
}

// Task is a unit of work with optional dependencies
type Task struct {
	Name      string
	Needs     []string
	Retry     RetryConfig
	ExecuteFn func(ctx context.Context, name string, handler OutputHandler) error
}

// RetryConfig controls retry behaviour for a task
type RetryConfig struct {
	Attempts int    // total attempts (1 = no retry)
	Delay    string // e.g. "5s"
	Backoff  string // "none", "linear", "exponential"
}

// taskStatus represents the execution status of a task
type taskStatus int

const (
	statusPending taskStatus = iota
	statusRunning
	statusSuccess
	statusFailed
)

// taskRun holds runtime state for a task
type taskRun struct {
	task      Task
	status    taskStatus
	attempt   int // current attempt (1-based)
	startedAt time.Time
}

// CommandResult is the result of a single task execution
type CommandResult struct {
	Name    string
	Success bool
	Error   error
}
