package executor

import (
	"fmt"
	"strings"
)

// ValidateNoCycles checks the task graph for dependency cycles using Kahn's algorithm.
// Returns an error describing the cycle if one is found.
func ValidateNoCycles(tasks []Task) error {
	// Build adjacency and in-degree maps
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // node -> list of nodes that depend on it

	taskSet := make(map[string]bool)
	for _, t := range tasks {
		taskSet[t.Name] = true
		if _, ok := inDegree[t.Name]; !ok {
			inDegree[t.Name] = 0
		}
	}

	for _, t := range tasks {
		for _, dep := range t.Needs {
			if !taskSet[dep] {
				return fmt.Errorf("task %q has unknown dependency %q", t.Name, dep)
			}
			inDegree[t.Name]++
			dependents[dep] = append(dependents[dep], t.Name)
		}
	}

	// Kahn's BFS
	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	processed := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		processed++

		for _, dependent := range dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if processed != len(tasks) {
		// Find cycle members (nodes with remaining in-degree > 0)
		var cycleMembers []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycleMembers = append(cycleMembers, name)
			}
		}
		return fmt.Errorf("dependency cycle detected among tasks: %s", strings.Join(cycleMembers, ", "))
	}

	return nil
}

// TopologicalSort returns tasks in a valid execution order (Kahn's algorithm)
func TopologicalSort(tasks []Task) ([]Task, error) {
	if err := ValidateNoCycles(tasks); err != nil {
		return nil, err
	}

	taskMap := make(map[string]Task)
	for _, t := range tasks {
		taskMap[t.Name] = t
	}

	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, t := range tasks {
		if _, ok := inDegree[t.Name]; !ok {
			inDegree[t.Name] = 0
		}
		for _, dep := range t.Needs {
			inDegree[t.Name]++
			dependents[dep] = append(dependents[dep], t.Name)
		}
	}

	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []Task
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, taskMap[node])

		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	return sorted, nil
}
