package executor

import "fmt"

type Command struct {
	name    string
	command string
	args    map[string]interface{}
	cwd     string
	needs   []string
}

type status int

const (
	commandPending status = iota
	commandRunning
	commandSuccess
	commandFailed
)

type CommandRun struct {
	command Command
	status
}

type Executor struct {
	commands map[string]*CommandRun
}

type CommandResult struct {
	CommandName string
	Success     bool
	Error       error
}

func NewExecutor(commands []Command) *Executor {
	commandRuns := make(map[string]*CommandRun)
	for i, cmd := range commands {
		commandRuns[cmd.name] = &CommandRun{
			command: cmd,
			status:  commandPending,
		}
	}

	return &Executor{
		commands: commandRuns,
	}
}

func (e *Executor) Execute() error {
	// Make channel for receiving command completion notifications
	completionCh := make(chan CommandResult)

	// Find all commands without dependencies, and start executing them
	err := e.triggerExecutionOfReadyCommands(completionCh)
	if err != nil {
		return err
	}

	if err := e.ensureAtLeastOneCommandRunning(); err != nil {
		return err
	}

	// Wait for any command to finish. If a command finishes, check for new commands that can be executed until all commands are done.
	finishedCommands := 0
	totalCommands := len(e.commands)
	for finishedCommands < totalCommands {
		// Wait for a command to complete
		result := <-completionCh
		finishedCommands++

		// Update command status
		cmdRun, exists := e.commands[result.CommandName]
		if !exists {
			return fmt.Errorf("received completion for unknown command: %s", result.CommandName)
		}

		if result.Success {
			cmdRun.status = commandSuccess
		} else {
			cmdRun.status = commandFailed
		}

		// Check for new commands that can be executed
		err := e.triggerExecutionOfReadyCommands(completionCh)
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

	return nil
}

// triggerExecutionOfReadyCommands finds and starts execution of all commands whose dependencies are met.
func (e *Executor) triggerExecutionOfReadyCommands(completionCh chan<- CommandResult) error {
	for name, cmdRun := range e.commands {
		if cmdRun.status != commandPending {
			continue
		}

		// Check if all dependencies are met
		depsMet := true
		for _, depName := range cmdRun.command.needs {
			depCmdRun, exists := e.commands[depName]
			if !exists {
				return fmt.Errorf("command %s has unknown dependency: %s", name, depName)
			}
			if depCmdRun.status != commandSuccess {
				depsMet = false
				break
			}
		}

		if depsMet {
			// Start executing the command
			cmdRun.status = commandRunning
			go func(cmd Command) {
				res, err := ExecuteSyncInDirectory(cmd.cwd, cmd.command, ...cmd.args)

				if err != nil || res.ExitCode != 0 {
					completionCh <- CommandResult{
						CommandName: cmd.name,
						Success:     false,
						Error:       fmt.Errorf("command %s failed: %v)", cmd.name, err),
					}
					return
				}
				completionCh <- CommandResult{
					CommandName: cmd.name,
					Success:     true,
					Error:       nil,
				}
			}(cmdRun.command)
		}
	}

	return nil
}

func (e *Executor) ensureAtLeastOneCommandRunning() error {
	for _, cmdRun := range e.commands {
		if cmdRun.status == commandRunning {
			return nil
		}
	}

	return fmt.Errorf("no commands are running, potential deadlock detected. Pending commands: %v", e.getPendingCommands())
}

// getPendingCommands returns a list of names of commands that are still pending.
func (e *Executor) getPendingCommands() []string {
	var pending []string
	for name, cmdRun := range e.commands {
		if cmdRun.status == commandPending {
			pending = append(pending, name)
		}
	}
	return pending
}
