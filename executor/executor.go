package executor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ExecutorOptions configures the executor
type ExecutorOptions struct {
	MaxParallelization int // 0 = unlimited

	// OnTaskStart is called just before a task begins executing.
	OnTaskStart func(name string)

	// OnTaskFinish is called when a task succeeds, fails, or is skipped.
	// err is nil on success.
	OnTaskFinish func(name string, err error, elapsed time.Duration)
}

// Executor runs tasks with dependency resolution, parallelization, and retry
type Executor struct {
	tasks         map[string]*taskRun
	outputHandler OutputHandler
	opts          ExecutorOptions
}

// NewExecutor creates an Executor from a list of tasks
func NewExecutor(tasks []Task, opts ExecutorOptions) (*Executor, error) {
	if err := ValidateNoCycles(tasks); err != nil {
		return nil, err
	}

	runs := make(map[string]*taskRun, len(tasks))
	for _, t := range tasks {
		runs[t.Name] = &taskRun{
			task:    t,
			status:  statusPending,
			attempt: 0,
		}
	}

	return &Executor{
		tasks:         runs,
		outputHandler: NoopOutputHandler{},
		opts:          opts,
	}, nil
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

	go e.drainOutput(ctx, outputCh)

	finished := 0
	total := len(e.tasks)
	var errs []error

	for finished < total {
		result := <-completionCh

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
				// Schedule retry
				delay := parseRetryDelay(run.task.Retry.Delay, run.task.Retry.Backoff, run.attempt)
				go func(r *taskRun, d time.Duration) {
					time.Sleep(d)
					r.status = statusPending
					_ = e.startReadyTasks(ctx, completionCh, outputCh, semaphore)
				}(run, delay)
			} else {
				run.status = statusFailed
				finished++
				if result.Error != nil {
					errs = append(errs, result.Error)
				}
			}
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
	return errors.Join(errs...)
}

// startReadyTasks finds all pending tasks whose deps are satisfied and starts them
func (e *Executor) startReadyTasks(ctx context.Context, completionCh chan<- CommandResult, outputCh chan<- Output, sem chan struct{}) error {
	for name, run := range e.tasks {
		if run.status != statusPending {
			continue
		}

		if !e.depsSatisfied(name) {
			continue
		}

		// Check if any dep failed (skip this task permanently)
		if e.depsHaveFailed(name) {
			run.status = statusFailed
			skipErr := fmt.Errorf("task %q skipped: dependency failed", name)
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
		case <-ctx.Done():
			return
		case out, ok := <-outputCh:
			if !ok {
				return
			}
			_ = e.outputHandler.HandleOutput(ctx, out)
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
