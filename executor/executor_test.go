package executor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecutor_BasicExecution(t *testing.T) {
	var count int32
	tasks := []Task{
		{
			Name: "task1",
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				atomic.AddInt32(&count, 1)
				return nil
			},
		},
		{
			Name: "task2",
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				atomic.AddInt32(&count, 1)
				return nil
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := exec.Execute(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 tasks to run, got %d", count)
	}
}

func TestExecutor_DependencyOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	tasks := []Task{
		{
			Name: "base",
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
				return nil
			},
		},
		{
			Name:  "dep",
			Needs: []string{"base"},
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
				return nil
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := exec.Execute(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(order) != 2 || order[0] != "base" || order[1] != "dep" {
		t.Errorf("wrong execution order: %v", order)
	}
}

func TestExecutor_MaxParallelization(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32

	tasks := make([]Task, 5)
	for i := range tasks {
		tasks[i] = Task{
			Name: string(rune('a' + i)),
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				current := atomic.AddInt32(&concurrent, 1)
				for {
					cur := atomic.LoadInt32(&maxConcurrent)
					if current <= cur || atomic.CompareAndSwapInt32(&maxConcurrent, cur, current) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
				return nil
			},
		}
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{MaxParallelization: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := exec.Execute(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if maxConcurrent > 2 {
		t.Errorf("max concurrent tasks exceeded limit: %d > 2", maxConcurrent)
	}
}

func TestExecutor_TaskFailureSkipsDependents(t *testing.T) {
	var depRan bool
	tasks := []Task{
		{
			Name: "base",
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				return errors.New("base failed")
			},
		},
		{
			Name:  "dep",
			Needs: []string{"base"},
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				depRan = true
				return nil
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runErr := exec.Execute(context.Background())
	if runErr == nil {
		t.Error("expected error from failing task, got nil")
	}
	if depRan {
		t.Error("dependent task should not run when its dependency failed")
	}
}

func TestExecutor_SkippedErrorWrapsErrTaskSkipped(t *testing.T) {
	var skippedErr error
	tasks := []Task{
		{
			Name: "base",
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				return errors.New("base failed")
			},
		},
		{
			Name:  "dep",
			Needs: []string{"base"},
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				return nil
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{
		OnTaskFinish: func(name string, err error, _ time.Duration) {
			if name == "dep" {
				skippedErr = err
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec.Execute(context.Background()) //nolint:errcheck
	if !errors.Is(skippedErr, ErrTaskSkipped) {
		t.Errorf("expected ErrTaskSkipped, got %v", skippedErr)
	}
}

func TestExecutor_RetrySucceedsOnSecondAttempt(t *testing.T) {
	var attempts int32
	tasks := []Task{
		{
			Name:  "flaky",
			Retry: RetryConfig{Attempts: 2, Delay: "0s"},
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				if atomic.AddInt32(&attempts, 1) < 2 {
					return errors.New("transient failure")
				}
				return nil
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := exec.Execute(context.Background()); err != nil {
		t.Errorf("expected success after retry, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestExecutor_RetryExhaustedReturnsError(t *testing.T) {
	var attempts int32
	tasks := []Task{
		{
			Name:  "broken",
			Retry: RetryConfig{Attempts: 3, Delay: "0s"},
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				atomic.AddInt32(&attempts, 1)
				return errors.New("always fails")
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := exec.Execute(context.Background()); err == nil {
		t.Error("expected error after all retry attempts exhausted")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestExecutor_ParseRetryDelay(t *testing.T) {
	cases := []struct {
		delay   string
		backoff string
		attempt int
		want    time.Duration
	}{
		{"0s", "none", 1, 0},
		{"10s", "none", 1, 10 * time.Second},
		{"10s", "none", 3, 10 * time.Second},
		{"2s", "linear", 1, 4 * time.Second},
		{"2s", "linear", 2, 6 * time.Second},
		{"2s", "exponential", 1, 4 * time.Second},
		{"2s", "exponential", 2, 8 * time.Second},
		{"", "none", 1, 5 * time.Second}, // default
	}

	for _, tc := range cases {
		got := parseRetryDelay(tc.delay, tc.backoff, tc.attempt)
		if got != tc.want {
			t.Errorf("parseRetryDelay(%q, %q, %d) = %v, want %v",
				tc.delay, tc.backoff, tc.attempt, got, tc.want)
		}
	}
}

func TestExecutor_DependencyBlocking(t *testing.T) {
	// dep must not start before base completes
	baseDone := make(chan struct{})
	var depStartedBeforeBase bool

	tasks := []Task{
		{
			Name: "base",
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				time.Sleep(20 * time.Millisecond)
				close(baseDone)
				return nil
			},
		},
		{
			Name:  "dep",
			Needs: []string{"base"},
			ExecuteFn: func(ctx context.Context, name string, h OutputHandler) error {
				select {
				case <-baseDone:
					// correct — base already done
				default:
					depStartedBeforeBase = true
				}
				return nil
			},
		},
	}

	exec, err := NewExecutor(tasks, ExecutorOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := exec.Execute(context.Background()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if depStartedBeforeBase {
		t.Error("dep started before base completed")
	}
}
