package executor

import (
	"testing"
)

func TestValidateNoCycles_NoCycle(t *testing.T) {
	tasks := []Task{
		{Name: "a"},
		{Name: "b", Needs: []string{"a"}},
		{Name: "c", Needs: []string{"a", "b"}},
	}
	if err := ValidateNoCycles(tasks); err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}
}

func TestValidateNoCycles_WithCycle(t *testing.T) {
	tasks := []Task{
		{Name: "a", Needs: []string{"b"}},
		{Name: "b", Needs: []string{"a"}},
	}
	if err := ValidateNoCycles(tasks); err == nil {
		t.Error("expected cycle error, got nil")
	}
}

func TestValidateNoCycles_SelfCycle(t *testing.T) {
	tasks := []Task{
		{Name: "a", Needs: []string{"a"}},
	}
	if err := ValidateNoCycles(tasks); err == nil {
		t.Error("expected self-cycle error, got nil")
	}
}

func TestValidateNoCycles_UnknownDep(t *testing.T) {
	tasks := []Task{
		{Name: "a", Needs: []string{"unknown"}},
	}
	if err := ValidateNoCycles(tasks); err == nil {
		t.Error("expected error for unknown dep, got nil")
	}
}

func TestTopologicalSort_Order(t *testing.T) {
	tasks := []Task{
		{Name: "c", Needs: []string{"b"}},
		{Name: "a"},
		{Name: "b", Needs: []string{"a"}},
	}
	sorted, err := TopologicalSort(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(sorted))
	}

	// Build position map
	pos := make(map[string]int)
	for i, t := range sorted {
		pos[t.Name] = i
	}

	if pos["a"] >= pos["b"] {
		t.Errorf("a should come before b, got positions a=%d b=%d", pos["a"], pos["b"])
	}
	if pos["b"] >= pos["c"] {
		t.Errorf("b should come before c, got positions b=%d c=%d", pos["b"], pos["c"])
	}
}
