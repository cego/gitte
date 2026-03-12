package config

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"
)

// ResolveTemplates applies template inheritance (extends) to all projects
// and performs Go text/template variable substitution.
// It modifies cfg.Projects in place and removes cfg.Templates when done.
func ResolveTemplates(cfg *GitteConfig) error {
	if cfg.Templates != nil {
		if err := resolveTemplateInheritance(cfg.Templates); err != nil {
			return err
		}
	}

	// Validate project extends references
	for projName, proj := range cfg.Projects {
		if proj.Extends == "" {
			continue
		}
		if cfg.Templates == nil {
			return fmt.Errorf("project %q extends template %q but no templates are defined", projName, proj.Extends)
		}
		if _, ok := cfg.Templates[proj.Extends]; !ok {
			return fmt.Errorf("project %q extends unknown template %q", projName, proj.Extends)
		}
	}

	if cfg.Templates == nil {
		return nil
	}

	for projName, proj := range cfg.Projects {
		if proj.Extends == "" {
			continue
		}

		tmpl, ok := cfg.Templates[proj.Extends]
		if !ok {
			return fmt.Errorf("project %q extends unknown template %q", projName, proj.Extends)
		}

		resolved, err := applyTemplate(projName, proj, tmpl)
		if err != nil {
			return fmt.Errorf("project %q template resolution failed: %w", projName, err)
		}
		cfg.Projects[projName] = resolved
	}

	// Strip templates section from config
	cfg.Templates = nil
	return nil
}

// resolveTemplateInheritance resolves extends within templates themselves,
// modifying the map in place. Templates are processed in topological order
// so parents are fully resolved before their children.
func resolveTemplateInheritance(templates map[string]Template) error {
	// Validate all extends references
	for name, tmpl := range templates {
		for _, parent := range tmpl.Extends {
			if _, ok := templates[parent]; !ok {
				return fmt.Errorf("template %q extends unknown template %q", name, parent)
			}
		}
	}

	// Topological sort (Kahn's algorithm)
	inDegree := make(map[string]int, len(templates))
	dependents := make(map[string][]string) // parent → children
	for name := range templates {
		inDegree[name] = 0
	}
	for name, tmpl := range templates {
		for _, parent := range tmpl.Extends {
			inDegree[name]++
			dependents[parent] = append(dependents[parent], name)
		}
	}

	queue := make([]string, 0, len(templates))
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	order := make([]string, 0, len(templates))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		order = append(order, name)
		for _, child := range dependents[name] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}
	if len(order) != len(templates) {
		return fmt.Errorf("cycle detected in template extends")
	}

	// Resolve each template in topological order (parents guaranteed resolved first)
	for _, name := range order {
		tmpl := templates[name]
		if len(tmpl.Extends) == 0 {
			continue
		}
		// Merge parents left to right, then apply own definitions on top
		merged := Template{}
		for _, parentName := range tmpl.Extends {
			merged = mergeTemplates(merged, templates[parentName])
		}
		merged = mergeTemplates(merged, Template{Vars: tmpl.Vars, Env: tmpl.Env, Actions: tmpl.Actions})
		merged.Extends = nil
		templates[name] = merged
	}

	return nil
}

// mergeTemplates merges src on top of dst: src vars/groups override dst for the same key,
// src needs replaces dst needs if src needs is non-nil (including empty slice).
func mergeTemplates(dst, src Template) Template {
	vars := make(map[string]string)
	for k, v := range dst.Vars {
		vars[k] = v
	}
	for k, v := range src.Vars {
		vars[k] = v
	}

	env := make(map[string]string)
	for k, v := range dst.Env {
		env[k] = v
	}
	for k, v := range src.Env {
		env[k] = v
	}

	actions := make(map[string]ProjectAction)
	for k, v := range dst.Actions {
		actions[k] = v
	}
	for actionName, srcAction := range src.Actions {
		if dstAction, exists := actions[actionName]; exists {
			mergedGroups := make(map[string][]string)
			for k, v := range dstAction.Groups {
				mergedGroups[k] = v
			}
			for k, v := range srcAction.Groups {
				mergedGroups[k] = v
			}
			needs := dstAction.Needs
			if srcAction.Needs != nil {
				needs = srcAction.Needs
			}
			retry := dstAction.Retry
			if srcAction.Retry != nil {
				retry = srcAction.Retry
			}
			mergedSearchFors := append(append([]SearchFor{}, dstAction.SearchFors...), srcAction.SearchFors...)
			actions[actionName] = ProjectAction{
				SearchFors: mergedSearchFors,
				Needs:      needs,
				Groups:     mergedGroups,
				Retry:      retry,
			}
		} else {
			actions[actionName] = srcAction
		}
	}

	result := Template{}
	if len(vars) > 0 {
		result.Vars = vars
	}
	if len(env) > 0 {
		result.Env = env
	}
	if len(actions) > 0 {
		result.Actions = actions
	}
	return result
}

// applyTemplate merges a template into a project config and applies variable substitution
func applyTemplate(projName string, proj ProjectConfig, tmpl Template) (ProjectConfig, error) {
	// Build vars: template.vars merged with project.vars (project wins)
	vars := make(map[string]string)
	for k, v := range tmpl.Vars {
		vars[k] = v
	}
	for k, v := range proj.Vars {
		vars[k] = v
	}

	// Add auto-vars
	vars["project"] = projName
	vars["remote"] = proj.Remote

	// Render vars themselves (vars can reference auto-vars like {{.project}}).
	// Sort keys for deterministic rendering order.
	varKeys := make([]string, 0, len(vars))
	for k := range vars {
		varKeys = append(varKeys, k)
	}
	sort.Strings(varKeys)
	for _, k := range varKeys {
		rendered, err := renderTemplate(vars[k], vars)
		if err != nil {
			// If rendering fails (e.g. circular ref), keep original value
			continue
		}
		vars[k] = rendered
	}

	// Merge actions: start with template actions, overlay project actions
	mergedActions := make(map[string]ProjectAction)
	for actionName, tmplAction := range tmpl.Actions {
		mergedActions[actionName] = tmplAction
	}
	for actionName, projAction := range proj.Actions {
		if tmplAction, exists := mergedActions[actionName]; exists {
			// Deep merge: project groups override template groups for same key
			mergedGroups := make(map[string][]string)
			for k, v := range tmplAction.Groups {
				mergedGroups[k] = v
			}
			for k, v := range projAction.Groups {
				mergedGroups[k] = v
			}

			// Project's needs replaces template's needs if non-nil (including empty slice)
			needs := tmplAction.Needs
			if projAction.Needs != nil {
				needs = projAction.Needs
			}

			// Project's retry replaces template's retry if specified
			retry := tmplAction.Retry
			if projAction.Retry != nil {
				retry = projAction.Retry
			}

			mergedSearchFors := append(append([]SearchFor{}, tmplAction.SearchFors...), projAction.SearchFors...)
			mergedActions[actionName] = ProjectAction{
				SearchFors: mergedSearchFors,
				Needs:      needs,
				Groups:     mergedGroups,
				Retry:      retry,
			}
		} else {
			mergedActions[actionName] = projAction
		}
	}

	// Apply variable substitution to all string values
	substitutedActions, err := substituteActionsVars(mergedActions, vars)
	if err != nil {
		return ProjectConfig{}, err
	}

	// Merge env: template env → project env (project wins)
	mergedEnv := make(map[string]string)
	for k, v := range tmpl.Env {
		mergedEnv[k] = v
	}
	for k, v := range proj.Env {
		mergedEnv[k] = v
	}

	result := proj
	result.Actions = substitutedActions
	result.Extends = "" // clear extends
	result.Vars = nil   // clear vars (already applied)
	if len(mergedEnv) > 0 {
		result.Env = mergedEnv
	} else {
		result.Env = nil
	}

	return result, nil
}

// substituteActionsVars applies Go text/template to all string values in actions
func substituteActionsVars(actions map[string]ProjectAction, vars map[string]string) (map[string]ProjectAction, error) {
	result := make(map[string]ProjectAction)
	for actionName, action := range actions {
		newGroups := make(map[string][]string)
		for groupName, cmds := range action.Groups {
			newCmds := make([]string, len(cmds))
			for i, cmd := range cmds {
				rendered, err := renderTemplate(cmd, vars)
				if err != nil {
					return nil, fmt.Errorf("action %q group %q cmd %q: %w", actionName, groupName, cmd, err)
				}
				newCmds[i] = rendered
			}
			newGroups[groupName] = newCmds
		}
		result[actionName] = ProjectAction{
			SearchFors: action.SearchFors,
			Needs:      action.Needs,
			Groups:     newGroups,
			Retry:      action.Retry,
		}
	}
	return result, nil
}

// renderTemplate applies Go text/template substitution to a string
func renderTemplate(text string, vars map[string]string) (string, error) {
	tmpl, err := template.New("").Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}
