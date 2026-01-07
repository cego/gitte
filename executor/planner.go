package executor

import (
	"context"
	"errors"
	"fmt"
)

type Task struct {
	Name      string
	ExecuteFn func(ctx context.Context, name string, handler OutputHandler) error
	Cwd       string
	Needs     []string
}

type status int

const (
	pending status = iota
	running
	success
	failed
)

type CommandRun struct {
	command Task
	status
}

type StreamType string

const (
	StderrStream StreamType = "stderr"
	StdoutStream StreamType = "stdout"
)

type Output struct {
	Output  []byte
	CmdName string
	Stream  StreamType
}

type OutputHandler interface {
	HandleOutput(ctx context.Context, output Output) error
}

type NoopOutputHandler struct{}

func (n NoopOutputHandler) HandleOutput(ctx context.Context, output Output) error {
	return nil
}

type LogOutputHandler struct{}

func (l LogOutputHandler) HandleOutput(ctx context.Context, output Output) error {
	prefix := fmt.Sprintf("[%s][%s]: ", output.CmdName, output.Stream)
	fmt.Printf("%s%s", prefix, string(output.Output))
	return nil
}

type Executor struct {
	tasks         map[string]*CommandRun
	outputHandler OutputHandler
}

type CommandResult struct {
	CommandName string
	Success     bool
	Error       error
}

func NewExecutor(commands []Task) *Executor {
	commandRuns := make(map[string]*CommandRun)
	for _, cmd := range commands {
		commandRuns[cmd.Name] = &CommandRun{
			command: cmd,
			status:  pending,
		}
	}

	return &Executor{
		tasks: commandRuns,
	}
}

func (e *Executor) WithOutputHandler(handler OutputHandler) *Executor {
	e.outputHandler = handler
	return e
}

func (e *Executor) Execute(ctx context.Context) error {
	// Make channel for receiving command completion notifications
	completionCh := make(chan CommandResult)
	outputChannel := make(chan Output, 1000) // Buffered channel to avoid blocking

	// Find all tasks without dependencies, and start executing them
	err := e.triggerExecutionOfReadyCommands(ctx, completionCh, outputChannel)
	if err != nil {
		return err
	}

	if err := e.ensureAtLeastOneCommandRunning(); err != nil {
		return err
	}

	go e.listenForOutput(ctx, outputChannel)

	// Wait for any command to finish. If a command finishes, check for new tasks that can be executed until all tasks are done.
	finishedCommands := 0
	totalCommands := len(e.tasks)
	errs := []error{}
	for finishedCommands < totalCommands {
		// Wait for a command to complete
		result := <-completionCh
		finishedCommands++

		// Update command status
		cmdRun, exists := e.tasks[result.CommandName]
		if !exists {
			return fmt.Errorf("received completion for unknown command: %s", result.CommandName)
		}

		if result.Success {
			cmdRun.status = success
		} else {
			cmdRun.status = failed
		}

		if result.Error != nil {
			errs = append(errs, result.Error)
		}

		// Check for new tasks that can be executed
		err := e.triggerExecutionOfReadyCommands(ctx, completionCh, outputChannel)
		if err != nil {
			return err
		}

		if finishedCommands == totalCommands {
			break
		}

		if err := e.ensureAtLeastOneCommandRunning(); err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}

func (e *Executor) listenForOutput(ctx context.Context, outputCh <-chan Output) {
	for {
		select {
		case <-ctx.Done():
			return
		case output, ok := <-outputCh:
			if !ok {
				return
			}
			if e.outputHandler.HandleOutput != nil {
				err := e.outputHandler.HandleOutput(ctx, output)
				if err != nil {
					fmt.Printf("error handling output for command %s: %v\n", output.CmdName, err)
				}
			}
		}
	}
}

type ToChannelOutputHandler struct {
	OutputCh chan<- Output
}

func (h ToChannelOutputHandler) HandleOutput(ctx context.Context, output Output) error {
	h.OutputCh <- output
	return nil
}

// triggerExecutionOfReadyCommands finds and starts execution of all tasks whose dependencies are met.
func (e *Executor) triggerExecutionOfReadyCommands(ctx context.Context, completionCh chan<- CommandResult, outputCh chan<- Output) error {
	for name, cmdRun := range e.tasks {
		if cmdRun.status != pending {
			continue
		}

		// Check if all dependencies are met
		depsMet := true
		for _, depName := range cmdRun.command.Needs {
			depCmdRun, exists := e.tasks[depName]
			if !exists {
				return fmt.Errorf("command %s has unknown dependency: %s", name, depName)
			}
			if depCmdRun.status != success {
				depsMet = false
				break
			}
		}

		if depsMet {
			// Start executing the command
			cmdRun.status = running
			go func(cmd Task) {
				outputHandler := ToChannelOutputHandler{OutputCh: outputCh}
				err := cmd.ExecuteFn(ctx, cmd.Name, outputHandler)

				if err != nil {
					completionCh <- CommandResult{
						CommandName: cmd.Name,
						Success:     false,
						Error:       fmt.Errorf("command %s failed: %v)", cmd.Name, err),
					}
					return
				}
				completionCh <- CommandResult{
					CommandName: cmd.Name,
					Success:     true,
					Error:       nil,
				}
			}(cmdRun.command)
		}
	}

	return nil
}

func (e *Executor) ensureAtLeastOneCommandRunning() error {
	for _, cmdRun := range e.tasks {
		if cmdRun.status == running {
			return nil
		}
	}

	return fmt.Errorf("no tasks are running, potential deadlock detected. Pending tasks: %v", e.getPendingCommands())
}

// getPendingCommands returns a list of names of tasks that are still pending.
func (e *Executor) getPendingCommands() []string {
	var pendingTasks []string
	for name, cmdRun := range e.tasks {
		if cmdRun.status == pending {
			pendingTasks = append(pendingTasks, name)
		}
	}
	return pendingTasks
}
