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

type ExecuteResult struct {
	ExitCode int8
	Stdout   []byte
	Stderr   []byte
}

// ExecuteSyncInDir executes a command synchronously in the specified directory (cwd).
// If cwd is an empty string, it executes in the current working directory.
// It returns an ExecuteResult containing the exit code, stdout, and stderr.
// It will only return an error if there was a problem starting or running the command itself. Not if the command exits with a non-zero exit code.
func ExecuteSyncInDir(cwd string, command string, args ...string) (*ExecuteResult, error) {
	cmd := exec.Command(command, args...)

	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// if the error is of type *exec.ExitError, we can get the exit code
	if err != nil {
		var exitCode int8
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = int8(exitError.ExitCode())
		}
		return &ExecuteResult{
			ExitCode: exitCode,
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
		}, nil
	}

	return &ExecuteResult{
		ExitCode: 0,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

func ExecuteSyncInDirWithOutputHandler(ctx context.Context, name string, cwd string, outputHandler OutputHandler, command string, args ...string) (*ExecuteResult, error) {
	cmd := exec.Command(command, args...)

	cmd.Dir = cwd
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	// Collect output while streaming
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var wg sync.WaitGroup

	// Handle stdout
	wg.Add(2)
	go func() {
		defer wg.Done()
		handleOutputStream(ctx, stdoutPipe, outputHandler, name, StdoutStream, &stdout)
	}()

	go func() {
		defer wg.Done()
		handleOutputStream(ctx, stderrPipe, outputHandler, name, StderrStream, &stderr)
	}()

	// Wait for goroutines to finish
	wg.Wait()

	// Wait for command to complete
	err = cmd.Wait()

	// Determine exit code
	var exitCode int8
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = int8(exitError.ExitCode())
		}
	}

	return &ExecuteResult{
		ExitCode: exitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

func handleOutputStream(ctx context.Context, pipe io.Reader, outputHandler OutputHandler, cmdName string, streamType StreamType, buffer *bytes.Buffer) {
	reader := bufio.NewReader(pipe)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return
		}
		if len(line) > 0 {
			_ = outputHandler.HandleOutput(ctx, Output{
				Output:  line,
				CmdName: cmdName,
				Stream:  streamType,
			})
			buffer.Write(line)
		}
		if err == io.EOF {
			break
		}
	}
}

func ExecuteSync(command string, args ...string) (*ExecuteResult, error) {
	return ExecuteSyncInDir("", command, args...)
}
