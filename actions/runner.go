package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"
	"github.com/cego/gitte/features"
	"github.com/cego/gitte/output"
	"github.com/cego/gitte/state"
)

// RunActions executes planned action tasks.
// mode controls whether to use plain or TUI output.
// envMaxParallel is the global parallelization cap from GITTE_MAX_TASK_PARALLELIZATION (0 = unlimited).
func RunActions(ctx context.Context, cfg *config.GitteConfig, st *state.GitteState, cwd string, mode output.OutputMode, keys []GroupKeyWithDeps, actionOrder []string, envMaxParallel int) error {
	runCtx, runCancel := context.WithCancel(ctx)

	// retryCh is shared between the view (writer) and executor (reader).
	// The user presses r/R while Execute is running → task names written here → executor re-queues inline.
	retryCh := make(chan []string, 100)

	infos := buildTaskInfos(cfg, st, cwd, keys)
	view := newView(mode, infos, actionOrder, runCancel, retryCh, cfg.QuickSolve.GitClean.Exclude)

	// Track per-task outcomes so retry runs can pre-complete tasks that already finished.
	outcomes := newTaskOutcomes()
	onFinish := func(name string, err error, elapsed time.Duration) {
		if err == nil {
			outcomes.set(name, outcomeSuccess)
		} else if errors.Is(err, executor.ErrTaskSkipped) {
			outcomes.set(name, outcomeSkipped)
		} else {
			outcomes.set(name, outcomeFailed)
		}
		view.OnFinish(name, err, elapsed)
	}

	maxParallel := envMaxParallel
	if len(keys) > 0 {
		actionName := keys[0].Action
		if override, ok := cfg.ActionOverride[actionName]; ok && override.MaxParallelization > 0 {
			maxParallel = override.MaxParallelization
		}
	}

	var retrySet map[string]struct{} // nil on first run
	var runErr error
	for {
		tasks := buildExecutorTasks(cfg, st, cwd, keys)

		// Strip needs from explicitly retried tasks so they run immediately.
		if retrySet != nil {
			for i, t := range tasks {
				if _, ok := retrySet[t.Name]; ok {
					tasks[i].Needs = nil
				}
			}
		}

		exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{
			MaxParallelization: maxParallel,
			OnTaskStart:        view.OnStart,
			OnTaskReset:        view.OnReset,
			OnTaskFinish:       onFinish,
		})
		if err != nil {
			runCancel()
			return fmt.Errorf("action planning failed: %w", err)
		}

		// On retry runs, pre-complete tasks that aren't being re-run.
		// This keeps the full task set in the executor so additional retries
		// submitted via retryCh can find any failed task.
		if retrySet != nil {
			snap := outcomes.snapshot()
			pendingSet := computePendingSet(retrySet, keys, snap)
			var succeeded, failed []string
			for _, t := range tasks {
				if pendingSet[t.Name] {
					continue
				}
				if snap[t.Name] == outcomeSuccess {
					succeeded = append(succeeded, t.Name)
				} else {
					failed = append(failed, t.Name)
				}
			}
			exec.WithPreCompleted(succeeded, failed)
		}

		exec.WithOutputHandler(view.Handler())
		exec.WithRetryChannel(retryCh)

		runErr = exec.Execute(runCtx)
		runCancel()

		retryNames := view.WaitAndGetRetry()
		if retryNames == nil {
			return runErr
		}

		retrySet = make(map[string]struct{}, len(retryNames))
		for _, n := range retryNames {
			retrySet[n] = struct{}{}
		}

		// Compute all tasks that should be reset to pending: retried tasks
		// plus skipped tasks that transitively depend on retried tasks.
		pendingNames := computePendingSetSlice(retrySet, keys, outcomes.snapshot())
		if len(pendingNames) == 0 {
			return runErr
		}

		retryCh = make(chan []string, 100)
		runCtx, runCancel = context.WithCancel(ctx)
		view.PrepareRetry(pendingNames, retryCh, runCancel)
	}
}

// buildTaskInfos constructs TaskInfo display metadata for each key.
func buildTaskInfos(cfg *config.GitteConfig, st *state.GitteState, cwd string, keys []GroupKeyWithDeps) []TaskInfo {
	infos := make([]TaskInfo, 0, len(keys))
	for _, key := range keys {
		proj, ok := cfg.Projects[key.Project]
		if !ok {
			continue
		}
		host, path, _, err := config.ParseRemoteURL(proj.Remote)
		if err != nil {
			host = "unknown"
			path = key.Project
		}
		segs := strings.Split(path, "/")
		var pathSegs []string
		projLeaf := path
		if len(segs) > 1 {
			pathSegs = segs[:len(segs)-1]
			projLeaf = segs[len(segs)-1]
		}

		var projectDir, localDir string
		if ld, lerr := config.LocalDirForRemote(proj.Remote); lerr == nil {
			localDir = ld
			projectDir = filepath.Join(cwd, ld)
		}

		var command string
		if action, ok := proj.Actions[key.Action]; ok {
			if cmds, ok := action.Groups[key.Group]; ok && len(cmds) > 0 {
				command = strings.Join(cmds, " ")
			}
		}

		// Merge project env + feature gate env for display.
		displayEnv := make(map[string]string)
		for k, v := range proj.Env {
			displayEnv[k] = v
		}
		for k, v := range extraEnvForProject(cfg, st, key.Project, proj) {
			displayEnv[k] = v
		}
		if len(displayEnv) == 0 {
			displayEnv = nil
		}

		infos = append(infos, TaskInfo{
			TaskName:      taskName(key.GroupKey),
			Project:       key.Project,
			Action:        key.Action,
			Group:         key.Group,
			Host:          host,
			PathSegs:      pathSegs,
			ProjLeaf:      projLeaf,
			ProjectDir:    projectDir,
			LocalDir:      localDir,
			Command:       command,
			ExtraEnv:      displayEnv,
			DefaultBranch: proj.DefaultBranch,
		})
	}
	return infos
}

// buildExecutorTasks constructs executor.Task list from keys.
func buildExecutorTasks(cfg *config.GitteConfig, st *state.GitteState, cwd string, keys []GroupKeyWithDeps) []executor.Task {
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

		needNames := make([]string, 0, len(key.Needs))
		for _, n := range key.Needs {
			needNames = append(needNames, taskName(n))
		}

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

		allSearchFors := make([]config.SearchFor, len(searchFors), len(searchFors)+len(action.SearchFors))
		copy(allSearchFors, searchFors)
		allSearchFors = append(allSearchFors, action.SearchFors...)

		tasks = append(tasks, executor.Task{
			Name:  taskName(key.GroupKey),
			Needs: needNames,
			Retry: retryConfig,
			ExecuteFn: func(ctx context.Context, tName string, handler executor.OutputHandler) error {
				return runGroupTask(ctx, cfg, st, cwd, proj, key.Project, tName, cmds, allSearchFors, handler)
			},
		})
	}
	return tasks
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
	projName string,
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

	env := buildEnv(cfg, st, projName, proj)

	emitTaskPreamble(ctx, handler, taskName, taskDir, cmds, proj, cfg, st, projName)

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

// emitTaskPreamble writes a short header to the task log showing the working
// directory, command, and any env vars injected by gitte (project env, env_when,
// feature gates). It is emitted before the command starts so the log is
// self-contained for debugging.
func emitTaskPreamble(
	ctx context.Context,
	handler executor.OutputHandler,
	taskName, taskDir string,
	cmds []string,
	proj config.ProjectConfig,
	cfg *config.GitteConfig,
	st *state.GitteState,
	projName string,
) {
	emit := func(line string) {
		_ = handler.HandleOutput(ctx, executor.Output{
			Output:  []byte(line),
			CmdName: taskName,
			Stream:  executor.StdoutStream,
		})
	}

	emit("  dir: " + taskDir)
	emit("  cmd: " + strings.Join(cmds, " "))

	// Collect only the vars gitte injects (not all of os.Environ).
	injected := make(map[string]string)
	for k, v := range proj.Env {
		injected[k] = v
	}
	for k, v := range config.ResolveEnvWhen(proj.EnvWhen, runtime.GOARCH) {
		injected[k] = v
	}
	for k, v := range extraEnvForProject(cfg, st, projName, proj) {
		injected[k] = v
	}
	if len(injected) > 0 {
		keys := make([]string, 0, len(injected))
		for k := range injected {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+injected[k])
		}
		emit("  env: " + strings.Join(parts, " "))
	}
}

// extraEnvForProject returns the env vars injected by feature gates for a project.
func extraEnvForProject(cfg *config.GitteConfig, st *state.GitteState, projName string, proj config.ProjectConfig) map[string]string {
	if st == nil || cfg.FeatureGates == nil {
		return nil
	}

	extra := make(map[string]string)
	for gateName, gate := range cfg.FeatureGates {
		fs, enabled := st.Features[gateName]
		if !enabled || !fs.Enabled {
			continue
		}

		if fs.OverrideScope != nil {
			host, path, _, err := config.ParseRemoteURL(proj.Remote)
			if err != nil {
				continue
			}
			if !features.ProjectMatchesOverrideScope(projName, host, path, fs.OverrideScope) {
				continue
			}
		} else if !ProjectMatchesScopeByName(projName, proj, gate.Scope) {
			continue
		}

		for k, v := range gate.Effects.Env {
			extra[k] = v
		}
		for k, v := range config.ResolveEnvWhen(gate.Effects.EnvWhen, runtime.GOARCH) {
			extra[k] = v
		}
	}

	if len(extra) == 0 {
		return nil
	}
	return extra
}

// buildEnv constructs the environment variables for a task, including feature gate injections
func buildEnv(cfg *config.GitteConfig, st *state.GitteState, projName string, proj config.ProjectConfig) []string {
	base := os.Environ()

	projEnv := proj.Env
	projEnvWhen := config.ResolveEnvWhen(proj.EnvWhen, runtime.GOARCH)
	featureEnv := extraEnvForProject(cfg, st, projName, proj)

	if len(projEnv) == 0 && len(projEnvWhen) == 0 && len(featureEnv) == 0 {
		return base
	}

	envMap := make(map[string]string, len(base))
	for _, kv := range base {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	// Project/template env (lowest priority after OS env)
	for k, v := range projEnv {
		envMap[k] = v
	}
	// env_when overrides env for the same key on matching arch; on non-matching
	// arch the key from env remains (or is absent if not in env).
	for k, v := range projEnvWhen {
		envMap[k] = v
	}
	// Feature gate env (highest priority, overrides project env)
	for k, v := range featureEnv {
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
		return true
	}

	host, path, _, err := config.ParseRemoteURL(proj.Remote)
	if err != nil {
		return false
	}

	for _, gs := range scope.GitlabGroups {
		if gs.Host == host && (path == gs.Group || strings.HasPrefix(path, gs.Group+"/")) {
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

type taskOutcome int

const (
	outcomeUnknown taskOutcome = iota
	outcomeSuccess
	outcomeFailed
	outcomeSkipped
)

// taskOutcomes is a concurrency-safe map of task name → outcome.
type taskOutcomes struct {
	mu sync.Mutex
	m  map[string]taskOutcome
}

func newTaskOutcomes() *taskOutcomes {
	return &taskOutcomes{m: make(map[string]taskOutcome)}
}

func (o *taskOutcomes) set(name string, outcome taskOutcome) {
	o.mu.Lock()
	o.m[name] = outcome
	o.mu.Unlock()
}

// snapshot returns a shallow copy safe for single-goroutine reads.
func (o *taskOutcomes) snapshot() map[string]taskOutcome {
	o.mu.Lock()
	cp := make(map[string]taskOutcome, len(o.m))
	for k, v := range o.m {
		cp[k] = v
	}
	o.mu.Unlock()
	return cp
}

// computePendingSet returns the set of task names that should be left as pending
// in a retry run: explicitly retried tasks plus skipped tasks that transitively
// depend on retried tasks.
func computePendingSet(retrySet map[string]struct{}, keys []GroupKeyWithDeps, outcomes map[string]taskOutcome) map[string]bool {
	pending := make(map[string]bool, len(retrySet))
	for name := range retrySet {
		pending[name] = true
	}

	// Build dependents map: task → tasks that depend on it.
	dependents := make(map[string][]string)
	for _, key := range keys {
		name := taskName(key.GroupKey)
		for _, need := range key.Needs {
			needName := taskName(need)
			dependents[needName] = append(dependents[needName], name)
		}
	}

	// Cascade to skipped dependents so they re-run when retried deps succeed.
	queue := make([]string, 0, len(retrySet))
	for name := range retrySet {
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, dep := range dependents[name] {
			if pending[dep] {
				continue
			}
			if outcomes[dep] == outcomeSkipped {
				pending[dep] = true
				queue = append(queue, dep)
			}
		}
	}

	return pending
}

func computePendingSetSlice(retrySet map[string]struct{}, keys []GroupKeyWithDeps, outcomes map[string]taskOutcome) []string {
	pending := computePendingSet(retrySet, keys, outcomes)
	result := make([]string, 0, len(pending))
	for name := range pending {
		result = append(result, name)
	}
	return result
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
		groups := re.FindStringSubmatch(line)
		if groups == nil {
			continue
		}
		expanded := expandHint(sf.Hint, groups)
		for _, hintLine := range strings.Split(expanded, "\n") {
			hintLine = strings.TrimSpace(hintLine)
			if hintLine == "" {
				continue
			}
			_ = h.inner.HandleOutput(ctx, executor.Output{
				CmdName: h.taskName,
				Stream:  executor.StdoutStream,
				Output:  []byte("[HINT] " + hintLine),
			})
		}
	}
	return h.inner.HandleOutput(ctx, out)
}

// expandHint expands capture group references ({1}, {2}, ...) and strips chalk-style
// color tags ({cyan text}, {bgYellow text}, etc.) inherited from the TypeScript config.
func expandHint(hint string, groups []string) string {
	// Expand {n} capture group references first.
	groupRef := regexp.MustCompile(`\{(\d+)\}`)
	hint = groupRef.ReplaceAllStringFunc(hint, func(m string) string {
		n, _ := strconv.Atoi(m[1 : len(m)-1])
		if n < len(groups) {
			return groups[n]
		}
		return ""
	})
	// Strip chalk-style color tags: {keyword content} → content.
	// Repeat until no more tags remain (handles nesting).
	chalkTag := regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9]*(?:\s[a-zA-Z0-9]*)*\s([^{}]*)\}`)
	for {
		replaced := chalkTag.ReplaceAllString(hint, "$1")
		if replaced == hint {
			break
		}
		hint = replaced
	}
	return strings.TrimSpace(hint)
}
