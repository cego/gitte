package executor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrTaskSkipped is wrapped in the error passed to OnTaskFinish when a task is
// skipped because one of its dependencies failed (the task itself never ran).
var ErrTaskSkipped = errors.New("dependency failed")

// ExecutorOptions configures the executor
type ExecutorOptions struct {
	MaxParallelization int // 0 = unlimited

	// OnTaskStart is called just before a task begins executing.
	OnTaskStart func(name string)

	// OnTaskFinish is called when a task succeeds, fails, or is skipped.
	// err is nil on success.
	OnTaskFinish func(name string, err error, elapsed time.Duration)

	// OnTaskReset is called when a task is re-queued for retry via RetryChannel.
	OnTaskReset func(name string)
}

// Executor runs tasks with dependency resolution, parallelization, and retry
type Executor struct {
	tasks         map[string]*taskRun
	dependents    map[string][]string // task name → names of tasks that depend on it
	outputHandler OutputHandler
	opts          ExecutorOptions
	retryReqCh    <-chan []string // receives batches of task names to re-queue mid-run
}

// NewExecutor creates an Executor from a list of tasks
func NewExecutor(tasks []Task, opts ExecutorOptions) (*Executor, error) {
	if err := ValidateNoCycles(tasks); err != nil {
		return nil, err
	}

	runs := make(map[string]*taskRun, len(tasks))
	deps := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		runs[t.Name] = &taskRun{
			task:    t,
			status:  statusPending,
			attempt: 0,
		}
		for _, need := range t.Needs {
			deps[need] = append(deps[need], t.Name)
		}
	}

	return &Executor{
		tasks:         runs,
		dependents:    deps,
		outputHandler: NoopOutputHandler{},
		opts:          opts,
	}, nil
}

// WithRetryChannel sets a channel the caller can write task names into to re-queue
// failed tasks while Execute is running. Skipped dependents are re-queued automatically.
func (e *Executor) WithRetryChannel(ch <-chan []string) *Executor {
	e.retryReqCh = ch
	return e
}

// WithOutputHandler sets the output handler for all tasks
func (e *Executor) WithOutputHandler(h OutputHandler) *Executor {
	e.outputHandler = h
	return e
}

// Execute runs all tasks respecting dependencies and returns the first error if any fail
func (e *Executor) Execute(ctx context.Context) error {
	completionCh := make(chan CommandResult, len(e.tasks)*2)
	outputCh := make(chan Output, 2000)
	// internalRetryCh receives tasks whose retry delay has elapsed, ready to be re-queued.
	// Using a channel ensures status mutations only happen on the main goroutine, avoiding data races.
	internalRetryCh := make(chan *taskRun, len(e.tasks))

	// Semaphore for max parallelization
	var semaphore chan struct{}
	if e.opts.MaxParallelization > 0 {
		semaphore = make(chan struct{}, e.opts.MaxParallelization)
	}

	if err := e.startReadyTasks(ctx, completionCh, outputCh, semaphore); err != nil {
		return err
	}

	if err := e.ensureProgress(); err != nil {
		return err
	}

	drainDone := make(chan struct{})
	go func() {
		e.drainOutput(ctx, outputCh)
		close(drainDone)
	}()

	finished := 0
	total := len(e.tasks)
	var errs []error

	for finished < total {
		retryReqCh := e.retryReqCh // nil disables the select case

		select {
		case result := <-completionCh:
			run, exists := e.tasks[result.Name]
			if !exists {
				return fmt.Errorf("completion for unknown task: %s", result.Name)
			}

			if result.Success {
				run.status = statusSuccess
				finished++
			} else {
				// Check retry
				maxAttempts := run.task.Retry.Attempts
				if maxAttempts < 1 {
					maxAttempts = 1
				}

				run.attempt++
				if run.attempt < maxAttempts {
					// Schedule retry: goroutine waits for the delay, then notifies the main loop
					// via internalRetryCh so all status mutations stay on the main goroutine.
					delay := parseRetryDelay(run.task.Retry.Delay, run.task.Retry.Backoff, run.attempt)
					go func(r *taskRun, d time.Duration) {
						time.Sleep(d)
						internalRetryCh <- r
					}(run, delay)
				} else {
					run.status = statusFailed
					finished++
					if result.Error != nil {
						errs = append(errs, result.Error)
					}
				}
			}

		case r := <-internalRetryCh:
			r.status = statusPending
			if e.opts.OnTaskReset != nil {
				e.opts.OnTaskReset(r.task.Name)
			}

		case names := <-retryReqCh:
			count := e.resetForRetry(names)
			finished -= count
		}

		if finished < total {
			if err := e.startReadyTasks(ctx, completionCh, outputCh, semaphore); err != nil {
				return err
			}
			if err := e.ensureProgress(); err != nil {
				return err
			}
		}
	}

	close(outputCh)
	<-drainDone // wait for all output to be processed before returning
	return errors.Join(errs...)
}

// startReadyTasks finds all pending tasks whose deps are satisfied and starts them
func (e *Executor) startReadyTasks(ctx context.Context, completionCh chan<- CommandResult, outputCh chan<- Output, sem chan struct{}) error {
	for name, run := range e.tasks {
		if run.status != statusPending {
			continue
		}

		// Check if any dep failed (skip this task permanently).
		// This must be checked before depsSatisfied because a failed dep also
		// causes depsSatisfied to return false, which would leave the task stuck.
		if e.depsHaveFailed(name) {
			run.status = statusFailed
			run.skipped = true
			skipErr := fmt.Errorf("task %q skipped: %w", name, ErrTaskSkipped)
			if e.opts.OnTaskFinish != nil {
				e.opts.OnTaskFinish(name, skipErr, 0)
			}
			completionCh <- CommandResult{
				Name:    name,
				Success: false,
				Error:   skipErr,
			}
			continue
		}

		if !e.depsSatisfied(name) {
			continue
		}

		run.status = statusRunning
		run.startedAt = time.Now()
		if e.opts.OnTaskStart != nil {
			e.opts.OnTaskStart(run.task.Name)
		}
		go func(r *taskRun) {
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			handler := ToChannelOutputHandler{OutputCh: outputCh}
			err := r.task.ExecuteFn(ctx, r.task.Name, handler)
			elapsed := time.Since(r.startedAt)

			if err != nil {
				if e.opts.OnTaskFinish != nil {
					e.opts.OnTaskFinish(r.task.Name, err, elapsed)
				}
				completionCh <- CommandResult{
					Name:    r.task.Name,
					Success: false,
					Error:   fmt.Errorf("task %s failed: %w", r.task.Name, err),
				}
				return
			}
			if e.opts.OnTaskFinish != nil {
				e.opts.OnTaskFinish(r.task.Name, nil, elapsed)
			}
			completionCh <- CommandResult{
				Name:    r.task.Name,
				Success: true,
			}
		}(run)
	}
	return nil
}

// resetForRetry re-queues the named failed tasks and cascades to any skipped dependents.
// Returns the number of tasks reset (caller must decrement its finished counter by this amount).
func (e *Executor) resetForRetry(names []string) int {
	toReset := make(map[string]bool, len(names))
	queue := make([]string, 0, len(names))
	for _, name := range names {
		run, ok := e.tasks[name]
		if ok && run.status == statusFailed && !run.skipped {
			toReset[name] = true
			queue = append(queue, name)
		}
	}
	// Cascade: also reset skipped dependents whose dep is being retried.
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, dep := range e.dependents[name] {
			depRun, ok := e.tasks[dep]
			if !ok || toReset[dep] || !depRun.skipped {
				continue
			}
			toReset[dep] = true
			queue = append(queue, dep)
		}
	}
	count := 0
	for name := range toReset {
		run := e.tasks[name]
		run.attempt = 0
		run.status = statusPending
		run.skipped = false
		run.startedAt = time.Time{}
		count++
		if e.opts.OnTaskReset != nil {
			e.opts.OnTaskReset(name)
		}
	}
	return count
}

func (e *Executor) depsSatisfied(name string) bool {
	run := e.tasks[name]
	for _, dep := range run.task.Needs {
		depRun, ok := e.tasks[dep]
		if !ok || depRun.status != statusSuccess {
			return false
		}
	}
	return true
}

func (e *Executor) depsHaveFailed(name string) bool {
	run := e.tasks[name]
	for _, dep := range run.task.Needs {
		depRun, ok := e.tasks[dep]
		if ok && depRun.status == statusFailed {
			return true
		}
	}
	return false
}

func (e *Executor) ensureProgress() error {
	for _, run := range e.tasks {
		if run.status == statusRunning || run.status == statusSuccess || run.status == statusFailed {
			return nil
		}
	}
	// Check if all remaining pending tasks are blocked by failed deps
	for _, run := range e.tasks {
		if run.status == statusPending {
			if !e.depsHaveFailed(run.task.Name) {
				return fmt.Errorf("deadlock: no tasks running but pending tasks exist: %s", e.pendingNames())
			}
		}
	}
	return nil
}

func (e *Executor) pendingNames() string {
	var names []string
	for name, run := range e.tasks {
		if run.status == statusPending {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func (e *Executor) drainOutput(ctx context.Context, outputCh <-chan Output) {
	for {
		select {
		case out, ok := <-outputCh:
			if !ok {
				return
			}
			_ = e.outputHandler.HandleOutput(ctx, out)
		case <-ctx.Done():
			// Drain any lines already queued before exiting so the last output of a
			// failing command is not lost when the context is cancelled.
			for {
				select {
				case out, ok := <-outputCh:
					if !ok {
						return
					}
					_ = e.outputHandler.HandleOutput(context.Background(), out)
				default:
					return
				}
			}
		}
	}
}

// parseRetryDelay computes the delay for a given attempt with the configured backoff
func parseRetryDelay(delayStr, backoff string, attempt int) time.Duration {
	base := parseDuration(delayStr)
	switch backoff {
	case "exponential":
		multiplier := time.Duration(1)
		for i := 0; i < attempt; i++ {
			multiplier *= 2
		}
		return base * multiplier
	case "linear":
		return base * time.Duration(attempt+1)
	default: // "none" or empty
		return base
	}
}

func parseDuration(s string) time.Duration {
	if s == "" {
		return 5 * time.Second
	}
	// Try to parse as integer seconds first
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "s")); err == nil {
		return time.Duration(n) * time.Second
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Second
	}
	return d
}
