package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a single config validation issue
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationResult holds all errors and warnings from validation
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (r *ValidationResult) AddError(field, msg string) {
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: msg})
}

func (r *ValidationResult) AddWarning(field, msg string) {
	r.Warnings = append(r.Warnings, ValidationError{Field: field, Message: msg})
}

func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// ValidateConfig performs structural validation on the config
func ValidateConfig(cfg *GitteConfig) *ValidationResult {
	result := &ValidationResult{}

	// Validate projects
	for name, project := range cfg.Projects {
		validateProject(result, cfg, name, project)
	}

	// Validate templates referenced by projects
	for name, project := range cfg.Projects {
		if project.Extends != "" {
			if _, ok := cfg.Templates[project.Extends]; !ok {
				result.AddError(
					fmt.Sprintf("projects.%s.extends", name),
					fmt.Sprintf("template %q not found", project.Extends),
				)
			}
		}
	}

	// Validate feature gate scopes
	for gateName, gate := range cfg.FeatureGates {
		for _, proj := range gate.Scope.Projects {
			if _, ok := cfg.Projects[proj]; !ok {
				result.AddWarning(
					fmt.Sprintf("feature_gates.%s.scope.projects", gateName),
					fmt.Sprintf("project %q not found in config", proj),
				)
			}
		}
	}

	// Detect cycles in action needs
	if cycles := detectNeedsCycles(cfg); len(cycles) > 0 {
		for _, cycle := range cycles {
			result.AddError("needs", fmt.Sprintf("cycle detected: %s", strings.Join(cycle, " → ")))
		}
	}

	// Validate env_when condition types in projects
	for projName, proj := range cfg.Projects {
		validateEnvWhen(result, fmt.Sprintf("projects.%s.env_when", projName), proj.EnvWhen)
	}

	// Validate env_when condition types in feature gates
	for gateName, gate := range cfg.FeatureGates {
		validateEnvWhen(result, fmt.Sprintf("feature_gates.%s.effects.env_when", gateName), gate.Effects.EnvWhen)
	}

	// Validate env_when condition types in templates
	for tmplName, tmpl := range cfg.Templates {
		validateEnvWhen(result, fmt.Sprintf("templates.%s.env_when", tmplName), tmpl.EnvWhen)
	}

	return result
}

func validateProject(result *ValidationResult, cfg *GitteConfig, name string, project ProjectConfig) {
	if project.Remote == "" {
		result.AddError(fmt.Sprintf("projects.%s.remote", name), "remote is required")
	}

	for actionName, action := range project.Actions {
		for _, need := range action.Needs {
			if _, ok := cfg.Projects[need]; !ok {
				result.AddError(
					fmt.Sprintf("projects.%s.actions.%s.needs", name, actionName),
					fmt.Sprintf("project %q referenced in needs not found", need),
				)
			}
		}
	}
}

// detectNeedsCycles finds cycles in the project action needs graph
// Returns a list of cycles (each cycle is a list of project names)
func detectNeedsCycles(cfg *GitteConfig) [][]string {
	// Build a graph: for each project and action, project depends on its needs
	// We simplify: for each action, project -> needs for that action
	// We only track inter-project dependencies (not intra-project)

	// Build adjacency for each action
	actionGraphs := make(map[string]map[string][]string) // action -> project -> needs

	for projName, proj := range cfg.Projects {
		for actionName, action := range proj.Actions {
			if _, ok := actionGraphs[actionName]; !ok {
				actionGraphs[actionName] = make(map[string][]string)
			}
			actionGraphs[actionName][projName] = action.Needs
		}
	}

	var allCycles [][]string
	for _, graph := range actionGraphs {
		cycles := findCyclesInGraph(graph)
		allCycles = append(allCycles, cycles...)
	}
	return allCycles
}

var knownConditionTypes = map[string]bool{
	"arch": true,
}

func validateEnvWhen(result *ValidationResult, field string, entries map[string]EnvWhenEntry) {
	for varName, entry := range entries {
		for i, c := range entry.Conditions {
			if !knownConditionTypes[c.Type] {
				result.AddError(
					fmt.Sprintf("%s.%s.conditions[%d].type", field, varName, i),
					fmt.Sprintf("unknown condition type %q; known types: arch", c.Type),
				)
			}
		}
	}
}

// findCyclesInGraph uses DFS to find cycles in a directed graph
func findCyclesInGraph(graph map[string][]string) [][]string {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var cycles [][]string

	var dfs func(node string, path []string)
	dfs = func(node string, path []string) {
		visited[node] = true
		inStack[node] = true
		path = append(path, node)

		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				dfs(neighbor, path)
			} else if inStack[neighbor] {
				// Found a cycle
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path[cycleStart:]))
					copy(cycle, path[cycleStart:])
					cycles = append(cycles, cycle)
				}
			}
		}

		inStack[node] = false
	}

	for node := range graph {
		if !visited[node] {
			dfs(node, []string{})
		}
	}

	return cycles
}
