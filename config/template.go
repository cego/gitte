package config

import (
	"bytes"
	"fmt"
	"text/template"
)

// ResolveTemplates applies template inheritance (extends) to all projects
// and performs Go text/template variable substitution.
// It modifies cfg.Projects in place and removes cfg.Templates when done.
func ResolveTemplates(cfg *GitteConfig) error {
	// Check for extends references even if Templates is nil
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

	// Render vars themselves (vars can reference auto-vars like {{.project}})
	for k, v := range vars {
		rendered, err := renderTemplate(v, vars)
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

			// Project's needs replaces template's needs if specified
			needs := tmplAction.Needs
			if len(projAction.Needs) > 0 {
				needs = projAction.Needs
			}

			// Project's retry replaces template's retry if specified
			retry := tmplAction.Retry
			if projAction.Retry != nil {
				retry = projAction.Retry
			}

			mergedActions[actionName] = ProjectAction{
				SearchFors: projAction.SearchFors,
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

	result := proj
	result.Actions = substitutedActions
	result.Extends = "" // clear extends
	result.Vars = nil   // clear vars (already applied)

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
