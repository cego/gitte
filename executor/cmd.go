package executor

import (
	"bytes"
	"errors"
	"os/exec"
)

type ExecuteResult struct {
	ExitCode int8
	Stdout   []byte
	Stderr   []byte
}

func ExecuteSync(command string, args ...string) (*ExecuteResult, error) {
	cmd := exec.Command(command, args...)
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
