package executor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
)

// ExecuteResult holds the result of a command execution
type ExecuteResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// ExecuteSyncInDir runs a command synchronously in the given directory
func ExecuteSyncInDir(ctx context.Context, cwd string, command string, args ...string) (*ExecuteResult, error) {
	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &ExecuteResult{
				ExitCode: exitErr.ExitCode(),
				Stdout:   stdout.Bytes(),
				Stderr:   stderr.Bytes(),
			}, nil
		}
		return nil, err
	}

	return &ExecuteResult{
		ExitCode: 0,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

// ExecuteSync runs a command synchronously in the current directory
func ExecuteSync(ctx context.Context, command string, args ...string) (*ExecuteResult, error) {
	return ExecuteSyncInDir(ctx, "", command, args...)
}

// ExecuteSyncInDirWithOutputHandler runs a command, streaming output via the handler
func ExecuteSyncInDirWithOutputHandler(
	ctx context.Context,
	name string,
	cwd string,
	outputHandler OutputHandler,
	env []string,
	command string,
	args ...string,
) (*ExecuteResult, error) {
	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec
	cmd.Dir = cwd

	if len(env) > 0 {
		cmd.Env = env
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// When context is cancelled, exec.CommandContext kills the parent process but
	// child processes may survive and hold the pipe file descriptors open, blocking
	// handleStream's ReadBytes forever. Close the pipes to unblock the readers.
	pipeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = stdoutPipe.Close()
			_ = stderrPipe.Close()
		case <-pipeDone:
		}
	}()

	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		handleStream(ctx, stdoutPipe, outputHandler, name, StdoutStream, &stdout)
	}()
	go func() {
		defer wg.Done()
		handleStream(ctx, stderrPipe, outputHandler, name, StderrStream, &stderr)
	}()

	wg.Wait()
	close(pipeDone)
	err = cmd.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	return &ExecuteResult{
		ExitCode: exitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

func handleStream(ctx context.Context, pipe io.Reader, handler OutputHandler, name string, stream StreamType, buf *bytes.Buffer) {
	reader := bufio.NewReader(pipe)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			_ = handler.HandleOutput(ctx, Output{
				Output:  line,
				CmdName: name,
				Stream:  stream,
			})
			buf.Write(line)
		}
		if err != nil {
			break
		}
	}
}
