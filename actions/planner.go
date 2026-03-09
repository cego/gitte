package actions

import (
	"strings"

	"gitte/config"

	"github.com/samber/lo"
)

// GroupKey uniquely identifies a project+action+group combination
type GroupKey struct {
	Project string
	Action  string
	Group   string
}

// GroupKeyWithDeps holds a GroupKey and its resolved dependencies
type GroupKeyWithDeps struct {
	GroupKey
	Needs []GroupKey
}

// PlanActions resolves which tasks to run based on the action/project/group strings
// and builds a list of GroupKeyWithDeps ready for the executor.
// It also returns the ordered list of action names (preserving the user's input order).
func PlanActions(cfg *config.GitteConfig, actionStr, projectStr, groupStr string, withNeeds bool) ([]GroupKeyWithDeps, []string) {
	actionList := parseGitteString(actionStr)
	projects := parseGitteString(projectStr)
	groups := parseGitteString(groupStr)

	// If no projects specified, default to "*"
	if len(projects) == 0 {
		projects = []string{"*"}
	}
	// If no groups specified, default to "*"
	if len(groups) == 0 {
		groups = []string{"*"}
	}

	selectedProjects := filterProjects(cfg, projects)
	projectActions := findProjectActions(cfg, selectedProjects, actionList)
	keys := findGroups(cfg, projectActions, groups)

	if withNeeds {
		keys = addDependencies(cfg, keys)
	}

	keys = removeUnrunnable(keys)
	keys = lo.UniqBy(keys, func(k GroupKeyWithDeps) string {
		return k.Project + "|" + k.Action + "|" + k.Group
	})
	keys = addActionOrderDeps(keys, actionList)

	return keys, actionList
}

// addActionOrderDeps ensures that all tasks of action[i] depend on all tasks of action[i-1].
// This prevents e.g. "up" from starting before all "build" tasks have succeeded.
func addActionOrderDeps(keys []GroupKeyWithDeps, actions []string) []GroupKeyWithDeps {
	if len(actions) <= 1 {
		return keys
	}
	actionTasks := make(map[string][]GroupKey, len(actions))
	for _, k := range keys {
		actionTasks[k.Action] = append(actionTasks[k.Action], k.GroupKey)
	}
	result := make([]GroupKeyWithDeps, len(keys))
	copy(result, keys)
	for i := 1; i < len(actions); i++ {
		prevTasks := actionTasks[actions[i-1]]
		if len(prevTasks) == 0 {
			continue
		}
		for j := range result {
			if result[j].Action != actions[i] {
				continue
			}
			existing := make(map[GroupKey]bool, len(result[j].Needs))
			for _, n := range result[j].Needs {
				existing[n] = true
			}
			for _, pt := range prevTasks {
				if !existing[pt] {
					result[j].Needs = append(result[j].Needs, pt)
				}
			}
		}
	}
	return result
}

func parseGitteString(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	for _, p := range strings.Split(s, "+") {
		p = strings.TrimSpace(p)
		if p == "all" {
			p = "*"
		}
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func filterProjects(cfg *config.GitteConfig, projectsStr []string) []string {
	keys := lo.Keys(cfg.Projects)

	if lo.Contains(projectsStr, "*") {
		return keys
	}

	return lo.Filter(keys, func(p string, _ int) bool {
		return lo.Contains(projectsStr, p)
	})
}

type projectAction struct {
	Project string
	Action  string
}

func findProjectActions(cfg *config.GitteConfig, projects []string, actionsStr []string) []projectAction {
	var result []projectAction
	for _, projName := range projects {
		proj, ok := cfg.Projects[projName]
		if !ok {
			continue
		}

		actionKeys := lo.Keys(proj.Actions)
		var filtered []string
		if lo.Contains(actionsStr, "*") {
			filtered = actionKeys
		} else {
			filtered = lo.Filter(actionKeys, func(a string, _ int) bool {
				return lo.Contains(actionsStr, a)
			})
		}

		for _, a := range filtered {
			result = append(result, projectAction{Project: projName, Action: a})
		}
	}
	return result
}

// expandGroups transitively adds included groups to the requested group list.
func expandGroups(groups []string, includes map[string][]string) []string {
	if len(includes) == 0 {
		return groups
	}
	seen := make(map[string]bool, len(groups))
	queue := make([]string, len(groups))
	copy(queue, groups)
	for _, g := range groups {
		seen[g] = true
	}
	expanded := make([]string, 0, len(groups))
	for len(queue) > 0 {
		g := queue[0]
		queue = queue[1:]
		expanded = append(expanded, g)
		for _, inc := range includes[g] {
			if !seen[inc] {
				seen[inc] = true
				queue = append(queue, inc)
			}
		}
	}
	return expanded
}

func findGroups(cfg *config.GitteConfig, pas []projectAction, groupsStr []string) []GroupKeyWithDeps {
	effectiveGroups := expandGroups(groupsStr, cfg.GroupIncludes)
	var result []GroupKeyWithDeps
	for _, pa := range pas {
		proj, ok := cfg.Projects[pa.Project]
		if !ok {
			continue
		}
		action, ok := proj.Actions[pa.Action]
		if !ok {
			continue
		}

		groupKeys := lo.Keys(action.Groups)
		var filtered []string
		if lo.Contains(groupsStr, "*") {
			filtered = groupKeys
		} else {
			filtered = lo.Filter(groupKeys, func(g string, _ int) bool {
				return lo.Contains(effectiveGroups, g) || g == "*"
			})
		}

		for _, g := range filtered {
			result = append(result, GroupKeyWithDeps{
				GroupKey: GroupKey{Project: pa.Project, Action: pa.Action, Group: g},
			})
		}
	}
	return result
}

// addDependencies resolves needs for all keys, adding dependency keys transitively
func addDependencies(cfg *config.GitteConfig, keys []GroupKeyWithDeps) []GroupKeyWithDeps {
	var all []GroupKeyWithDeps

	for _, key := range keys {
		all = append(all, key)
		all = append(all, collectDeps(cfg, key, all)...)
	}

	// Resolve needs references
	allMap := make(map[string]GroupKeyWithDeps)
	for _, k := range all {
		allMap[k.Project+"|"+k.Action+"|"+k.Group] = k
	}

	for i, k := range all {
		proj, ok := cfg.Projects[k.Project]
		if !ok {
			continue
		}
		action, ok := proj.Actions[k.Action]
		if !ok {
			continue
		}

		all[i].Needs = resolveNeeds(k.GroupKey, action.Needs, all, cfg.GroupIncludes)
	}

	return all
}

func collectDeps(cfg *config.GitteConfig, key GroupKeyWithDeps, existing []GroupKeyWithDeps) []GroupKeyWithDeps {
	proj, ok := cfg.Projects[key.Project]
	if !ok {
		return nil
	}
	action, ok := proj.Actions[key.Action]
	if !ok {
		return nil
	}

	var result []GroupKeyWithDeps
	for _, need := range action.Needs {
		depKey := findNeedKey(cfg, key.GroupKey, need)
		result = append(result, depKey)
		result = append(result, collectDeps(cfg, depKey, existing)...)
	}
	return result
}

func findNeedKey(cfg *config.GitteConfig, key GroupKey, need string) GroupKeyWithDeps {
	needProj, ok := cfg.Projects[need]
	if !ok {
		return GroupKeyWithDeps{GroupKey: GroupKey{Project: need, Action: key.Action, Group: "!"}}
	}
	needAction, ok := needProj.Actions[key.Action]
	if !ok {
		return GroupKeyWithDeps{GroupKey: GroupKey{Project: need, Action: key.Action, Group: "!"}}
	}
	// Prefer same group, then "*", then included groups
	if _, ok := needAction.Groups[key.Group]; ok {
		return GroupKeyWithDeps{GroupKey: GroupKey{Project: need, Action: key.Action, Group: key.Group}}
	}
	if _, ok := needAction.Groups["*"]; ok {
		return GroupKeyWithDeps{GroupKey: GroupKey{Project: need, Action: key.Action, Group: "*"}}
	}
	for _, inc := range cfg.GroupIncludes[key.Group] {
		if _, ok := needAction.Groups[inc]; ok {
			return GroupKeyWithDeps{GroupKey: GroupKey{Project: need, Action: key.Action, Group: inc}}
		}
	}
	return GroupKeyWithDeps{GroupKey: GroupKey{Project: need, Action: key.Action, Group: "!"}}
}

func resolveNeeds(key GroupKey, needs []string, all []GroupKeyWithDeps, includes map[string][]string) []GroupKey {
	effectiveGroups := expandGroups([]string{key.Group}, includes)
	var result []GroupKey
	for _, need := range needs {
		for _, k := range all {
			if k.Project == need && k.Action == key.Action &&
				(k.Group == "*" || lo.Contains(effectiveGroups, k.Group)) {
				result = append(result, k.GroupKey)
				break
			}
		}
	}
	return result
}

// removeUnrunnable removes keys with group "!" and fixes up dependencies
func removeUnrunnable(keys []GroupKeyWithDeps) []GroupKeyWithDeps {
	// First remove all "!" keys
	runnable := lo.Filter(keys, func(k GroupKeyWithDeps, _ int) bool {
		return k.Group != "!"
	})

	// Filter out "!" from needs
	for i := range runnable {
		runnable[i].Needs = lo.Filter(runnable[i].Needs, func(n GroupKey, _ int) bool {
			return n.Group != "!"
		})
	}

	return runnable
}
