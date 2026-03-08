package actions

import (
	"context"
	"fmt"
	"gitte/config"
	"gitte/executor"
	"gitte/state"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RunActions executes planned action tasks.
// envMaxParallel is the global parallelization cap from GITTE_MAX_TASK_PARALLELIZATION (0 = unlimited).
func RunActions(ctx context.Context, cfg *config.GitteConfig, st *state.GitteState, cwd string, keys []GroupKeyWithDeps, envMaxParallel int) error {
	tasks := make([]executor.Task, 0, len(keys))
	searchFors := cfg.SearchFor

	for _, key := range keys {
		key := key

		proj, ok := cfg.Projects[key.Project]
		if !ok {
			continue
		}
		action, ok := proj.Actions[key.Action]
		if !ok {
			continue
		}
		cmds, ok := action.Groups[key.Group]
		if !ok {
			continue
		}

		// Build needs list for executor
		needNames := make([]string, 0, len(key.Needs))
		for _, n := range key.Needs {
			needNames = append(needNames, taskName(n))
		}

		// Determine retry config
		retryConfig := executor.RetryConfig{Attempts: 1}
		if action.Retry != nil {
			retryConfig.Attempts = action.Retry.Attempts
			retryConfig.Delay = action.Retry.Delay
			retryConfig.Backoff = action.Retry.Backoff
		} else if cfg.Retry.Default.Attempts > 0 {
			retryConfig.Attempts = cfg.Retry.Default.Attempts
			retryConfig.Delay = cfg.Retry.Default.Delay
			retryConfig.Backoff = cfg.Retry.Default.Backoff
		}

		// Build per-action searchFors (combine global + action-specific)
		allSearchFors := append(searchFors, action.SearchFors...)

		tasks = append(tasks, executor.Task{
			Name:  taskName(key.GroupKey),
			Needs: needNames,
			Retry: retryConfig,
			ExecuteFn: func(ctx context.Context, tName string, handler executor.OutputHandler) error {
				return runGroupTask(ctx, cfg, st, cwd, proj, tName, cmds, allSearchFors, handler)
			},
		})
	}

	// Determine max parallelization: actionOverride config takes precedence, then env var
	maxParallel := envMaxParallel
	if len(keys) > 0 {
		actionName := keys[0].Action
		if override, ok := cfg.ActionOverride[actionName]; ok && override.MaxParallelization > 0 {
			maxParallel = override.MaxParallelization
		}
	}

	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{MaxParallelization: maxParallel})
	if err != nil {
		return fmt.Errorf("action planning failed: %w", err)
	}

	return exec.Execute(ctx)
}

func taskName(key GroupKey) string {
	return fmt.Sprintf("%s:%s:%s", key.Project, key.Action, key.Group)
}

func runGroupTask(
	ctx context.Context,
	cfg *config.GitteConfig,
	st *state.GitteState,
	cwd string,
	proj config.ProjectConfig,
	taskName string,
	cmds []string,
	searchFors []config.SearchFor,
	handler executor.OutputHandler,
) error {
	if len(cmds) == 0 {
		return fmt.Errorf("empty command for task %s", taskName)
	}

	localDir, err := config.LocalDirForRemote(proj.Remote)
	if err != nil {
		return err
	}

	taskDir := filepath.Join(cwd, localDir)

	// Build environment with feature gate injections
	env := buildEnv(cfg, st, proj)

	// Use a search-for output handler wrapper
	wrappedHandler := &searchForHandler{
		inner:      handler,
		searchFors: searchFors,
		taskName:   taskName,
	}

	res, err := executor.ExecuteSyncInDirWithOutputHandler(
		ctx, taskName, taskDir, wrappedHandler, env,
		cmds[0], cmds[1:]...,
	)
	if err != nil {
		return err
	}

	if res.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", res.ExitCode)
	}

	return nil
}

// buildEnv constructs the environment variables for a task, including feature gate injections
func buildEnv(cfg *config.GitteConfig, st *state.GitteState, proj config.ProjectConfig) []string {
	base := os.Environ()

	if st == nil || cfg.FeatureGates == nil {
		return base
	}

	extra := make(map[string]string)

	for gateName, gate := range cfg.FeatureGates {
		fs, enabled := st.Features[gateName]
		if !enabled || !fs.Enabled {
			continue
		}

		// Determine effective scope
		scope := gate.Scope
		if fs.OverrideScope != nil {
			scope = config.FeatureScope{
				Projects: fs.OverrideScope.Projects,
			}
		}

		if projectMatchesScope(proj, scope) {
			for k, v := range gate.Effects.Env {
				extra[k] = v
			}
		}
	}

	if len(extra) == 0 {
		return base
	}

	// Merge extra into base (extra wins)
	envMap := make(map[string]string, len(base))
	for _, kv := range base {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	for k, v := range extra {
		envMap[k] = v
	}

	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

// projectMatchesScope checks if a project is within a feature gate's scope
func projectMatchesScope(proj config.ProjectConfig, scope config.FeatureScope) bool {
	if len(scope.Projects) == 0 && len(scope.GitlabGroups) == 0 && len(scope.GithubOrgs) == 0 {
		return true // no scope restriction = applies to all
	}

	// Check direct project name match
	// (We don't have the project name here; the caller would need to pass it)
	// For now we check the remote URL against scopes

	host, path, _, err := config.ParseRemoteURL(proj.Remote)
	if err != nil {
		return false
	}

	for _, gs := range scope.GitlabGroups {
		if gs.Host == host && strings.HasPrefix(path, gs.Group) {
			return true
		}
	}

	for _, ghs := range scope.GithubOrgs {
		if ghs.Host == host && strings.HasPrefix(path, ghs.Org+"/") {
			return true
		}
	}

	return false
}

// ProjectMatchesScopeByName checks scope including project name list
func ProjectMatchesScopeByName(projName string, proj config.ProjectConfig, scope config.FeatureScope) bool {
	if len(scope.Projects) > 0 {
		for _, p := range scope.Projects {
			if p == projName {
				return true
			}
		}
	}
	return projectMatchesScope(proj, scope)
}

// searchForHandler wraps an OutputHandler and checks output lines against searchFor patterns
type searchForHandler struct {
	inner      executor.OutputHandler
	searchFors []config.SearchFor
	taskName   string
}

func (h *searchForHandler) HandleOutput(ctx context.Context, out executor.Output) error {
	line := string(out.Output)
	for _, sf := range h.searchFors {
		re, err := regexp.Compile(sf.Regex)
		if err != nil {
			continue
		}
		if re.MatchString(line) {
			hint := fmt.Sprintf("\n[HINT] %s\n", sf.Hint)
			_ = h.inner.HandleOutput(ctx, executor.Output{
				CmdName: h.taskName,
				Stream:  executor.StdoutStream,
				Output:  []byte(hint),
			})
		}
	}
	return h.inner.HandleOutput(ctx, out)
}
