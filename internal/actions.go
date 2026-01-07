package internal

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"
)

type ActionExecutor struct {
	Action   string
	Executor *executor.Executor
}

func CreateActionExecutors(cfg config.GitteConfig, withNeeds bool, actionString string, groupString string, projectString string) []ActionExecutor {
	actions := parseGitteString(actionString)
	groups := parseGitteString(groupString)
	projects := parseGitteString(projectString)

	result := make([]ActionExecutor, 0, len(actions))
	for _, action := range actions {
		exec := findGitteTasks(cfg, withNeeds, []string{action}, groups, projects)
		result = append(result, ActionExecutor{
			Action:   action,
			Executor: executor.NewExecutor(exec).WithOutputHandler(executor.LogOutputHandler{}),
		})
	}

	return result
}

func groupKeyToTask(cfg config.GitteConfig, key GroupKey) executor.Task {
	projectConfig, ok := cfg.Projects[key.Project]
	if !ok {
		return executor.Task{}
	}

	actionConfig, ok := projectConfig.Actions[key.Action]
	if !ok {
		return executor.Task{}
	}

	groupTasks, ok := actionConfig.Groups[key.Group]
	if !ok {
		return executor.Task{}
	}

	return executor.Task{
		Name: fmt.Sprintf("%s-%s-%s", key.Project, key.Action, key.Group),
		ExecuteFn: func(ctx context.Context, name string, oh executor.OutputHandler) error {
			dir, err := getProjectDirFromRemote(projectConfig)

			if err != nil {
				return err
			}

			cwd := config.CwdFromContext(ctx)

			res, err := executor.ExecuteSyncInDirWithOutputHandler(ctx, name, filepath.Join(cwd, dir), oh, groupTasks[0], groupTasks[1:]...)
			if err != nil {
				return err
			}

			if res.ExitCode != 0 {
				return fmt.Errorf("command failed with exit code %d: %s", res.ExitCode, string(res.Stderr))
			}

			fmt.Printf("Ran %s %s %s successfully (%s) in %s\n", key.Project, key.Action, key.Group, strings.Join(groupTasks, " "), filepath.Join(cwd, dir))

			return nil
		},
	}
}

func findGitteTasks(cfg config.GitteConfig, withNeeds bool, actionsStr, groupsStr, projectsStr []string) []executor.Task {
	projects := findProjects(cfg, projectsStr)
	projectActions := findProjectActions(cfg, projects, actionsStr)
	projectActionGroups := findGroups(cfg, projectActions, groupsStr)

	if withNeeds {
		projectActionGroups = addProjectDependencies(cfg, projectActionGroups)
	}

	fmt.Println("Count of project action groups before removing unrunnable:", len(projectActionGroups))

	projectActionGroups = removeUnrunnableGroups(projectActionGroups)

	fmt.Println("Count of project action groups after removing unrunnable:", len(projectActionGroups))

	// Deduplicate using compareGroupKeys
	projectActionGroups = lo.UniqBy(projectActionGroups, func(a GroupKeyWithDependencies) string {
		return a.Project + "|" + a.Action + "|" + a.Group
	})

	return lo.Map(projectActionGroups, func(key GroupKeyWithDependencies, _ int) executor.Task {
		return groupKeyToTask(cfg, key.GroupKey)
	})
}

func findProjects(cfg config.GitteConfig, projectsStr []string) []string {
	projectKeys := lo.Keys(cfg.Projects)

	if slices.Contains(projectsStr, "*") {
		return projectKeys
	}

	return lo.Filter(projectKeys, func(proj string, _ int) bool {
		return slices.Contains(projectsStr, proj)
	})
}

type projectAction struct {
	Project string
	Action  string
}

type GroupKey struct {
	projectAction
	Group string
}

type GroupKeyWithDependencies struct {
	GroupKey
	Needs []GroupKey
}

func findProjectActions(cfg config.GitteConfig, projects []string, actionsStr []string) []projectAction {
	// Find all actions possible for the given projects
	var projectActions []projectAction
	for _, projectName := range projects {
		projectConfig, ok := cfg.Projects[projectName]
		if !ok {
			continue
		}

		actionKeys := lo.Keys(projectConfig.Actions)
		var filteredActionKeys []string
		if slices.Contains(actionsStr, "*") {
			filteredActionKeys = actionKeys
		} else {
			filteredActionKeys = lo.Filter(actionKeys, func(action string, _ int) bool {
				return slices.Contains(actionsStr, action)
			})
		}

		for _, actionName := range filteredActionKeys {
			projectActions = append(projectActions, projectAction{
				Project: projectName,
				Action:  actionName,
			})
		}
	}

	return projectActions
}

func findGroups(cfg config.GitteConfig, projectActions []projectAction, groupsStr []string) []GroupKeyWithDependencies {
	var grouped []GroupKeyWithDependencies

	for _, pa := range projectActions {
		projectConfig, ok := cfg.Projects[pa.Project]
		if !ok {
			continue
		}

		actionConfig, ok := projectConfig.Actions[pa.Action]
		if !ok {
			continue
		}

		actionGroups := lo.Keys(actionConfig.Groups)
		var filteredActionGroups []string
		if slices.Contains(groupsStr, "*") {
			filteredActionGroups = actionGroups
		} else {
			filteredActionGroups = lo.Filter(actionGroups, func(group string, _ int) bool {
				return slices.Contains(groupsStr, group)
			})
		}

		for _, groupName := range filteredActionGroups {
			grouped = append(grouped, GroupKeyWithDependencies{
				GroupKey: GroupKey{
					projectAction: pa,
					Group:         groupName,
				},
			})
		}
	}

	return grouped
}

func parseGitteString(s string) []string {
	var parts []string
	for _, part := range strings.Split(s, "+") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}

	// replace any "all" with "*"
	for i, part := range parts {
		if part == "all" {
			parts[i] = "*"
		}
	}
	return parts
}

func findNeedGroupKeys(cfg config.GitteConfig, groupKey GroupKeyWithDependencies, needs []string) []GroupKeyWithDependencies {
	var neededGroupKeys []GroupKeyWithDependencies
	for _, need := range needs {
		project, ok := cfg.Projects[groupKey.Project]
		if !ok {
			continue
		}

		action, ok := project.Actions[groupKey.Action]
		if !ok {
			continue
		}

		// Find the current group's needs
		if _, ok := action.Groups[groupKey.Group]; ok {
			neededGroupKeys = append(neededGroupKeys, GroupKeyWithDependencies{
				GroupKey: GroupKey{
					projectAction: projectAction{
						Project: need,
						Action:  groupKey.Action,
					},
					Group: groupKey.Group,
				},
			})
			continue
		}

		// Check if star group exists
		if _, ok := action.Groups["*"]; ok {
			neededGroupKeys = append(neededGroupKeys, GroupKeyWithDependencies{
				GroupKey: GroupKey{
					projectAction: projectAction{
						Project: need,
						Action:  groupKey.Action,
					},
					Group: "*",
				},
			})
			continue
		}

		// Need not able to be solved. We mark this by using group "!"
		neededGroupKeys = append(neededGroupKeys, GroupKeyWithDependencies{
			GroupKey: GroupKey{
				projectAction: projectAction{
					Project: need,
					Action:  groupKey.Action,
				},
				Group: "!",
			},
		})
	}

	return neededGroupKeys
}

func resolveNeeds(key GroupKey, needs []string, keys []GroupKeyWithDependencies) []GroupKey {
	var neededGroupKeys []GroupKey

	for _, need := range needs {
		needKeySet := GroupKey{Group: key.Group, projectAction: projectAction{Project: need, Action: key.Action}}

		_, foundExact := lo.Find(keys, func(k GroupKeyWithDependencies) bool {
			return compareGroupKeys(k.GroupKey, needKeySet)
		})

		if foundExact {
			neededGroupKeys = append(neededGroupKeys, needKeySet)
			continue
		}

		// Check for star group
		needKeyStar := GroupKey{Group: "*", projectAction: projectAction{Project: need, Action: key.Action}}
		_, foundStar := lo.Find(keys, func(k GroupKeyWithDependencies) bool {
			return compareGroupKeys(k.GroupKey, needKeyStar)
		})

		if foundStar {
			neededGroupKeys = append(neededGroupKeys, needKeyStar)
			continue
		}

		// Mark as not found
		neededGroupKeys = append(neededGroupKeys, GroupKey{Group: "!", projectAction: projectAction{Project: need, Action: key.Action}})
	}

	return neededGroupKeys
}

func compareGroupKeys(a, b GroupKey) bool {
	return a.Project == b.Project && a.Action == b.Action && a.Group == b.Group
}

func addProjectDependencies(cfg config.GitteConfig, keys []GroupKeyWithDependencies) []GroupKeyWithDependencies {
	var foundKeys []GroupKeyWithDependencies

	for _, key := range keys {
		// first add the key itself
		foundKeys = append(foundKeys, key)

		foundKeys = append(foundKeys, addProjectDependenciesHelper(cfg, key, foundKeys)...)
	}

	// resolve the needs property for each key
	for _, key := range foundKeys {
		project, ok := cfg.Projects[key.Project]
		if !ok {
			continue
		}

		action, ok := project.Actions[key.Action]
		if !ok {
			continue
		}

		key.Needs = resolveNeeds(key.GroupKey, action.Needs, foundKeys)
	}

	return foundKeys
}

func addProjectDependenciesHelper(cfg config.GitteConfig, key GroupKeyWithDependencies, foundKeys []GroupKeyWithDependencies) []GroupKeyWithDependencies {
	project, ok := cfg.Projects[key.Project]
	if !ok {
		return foundKeys
	}

	action, ok := project.Actions[key.Action]
	if !ok {
		return foundKeys
	}

	needs := findNeedGroupKeys(cfg, key, action.Needs)

	var newGroups []GroupKeyWithDependencies
	for _, need := range needs {
		newGroups = append(newGroups, need)
		newGroups = append(newGroups, addProjectDependenciesHelper(cfg, need, foundKeys)...)
	}

	return newGroups
}

func removeUnrunnableGroups(keys []GroupKeyWithDependencies) []GroupKeyWithDependencies {
	var runnableKeys []GroupKeyWithDependencies
	runnableKeys = append(runnableKeys, keys...)

	for _, key := range keys {
		// If any of the needs has group "!", this group is unrunnable
		if key.Group != "!" {
			continue
		}

		// Remove the key from runnable keys
		runnableKeys = lo.Filter(runnableKeys, func(k GroupKeyWithDependencies, _ int) bool {
			return !compareGroupKeys(k.GroupKey, key.GroupKey)
		})

		// Replace the need on this key with the needs of the removed key
		dependentKeySets := lo.Filter(runnableKeys, func(k GroupKeyWithDependencies, _ int) bool {
			for _, need := range k.Needs {
				if compareGroupKeys(need, key.GroupKey) {
					return true
				}
			}
			return false
		})

		for _, dependentKeySet := range dependentKeySets {
			// Remove the need that is the removed key
			dependentKeySet.Needs = lo.Filter(dependentKeySet.Needs, func(need GroupKey, _ int) bool {
				return !compareGroupKeys(need, key.GroupKey)
			})

			// Add the needs of the removed key
			dependentKeySet.Needs = append(dependentKeySet.Needs, key.Needs...)
		}
	}

	return runnableKeys
}
