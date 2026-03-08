package executor

import (
	"context"
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
