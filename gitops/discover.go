package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"
	"github.com/cego/gitte/output"
)

// DiscoveredRepo represents a repo found via API discovery
type DiscoveredRepo struct {
	Remote string
	Host   string
	Path   string
}

// Discover fetches all repos from configured sources (GitLab groups + GitHub orgs)
// and clones/pulls them. Runs in two TUI phases: API discovery, then clone/pull.
func Discover(ctx context.Context, cfg *config.GitteConfig, cwd string, mode output.OutputMode, warnFn func(string)) error {
	type sourceEntry struct {
		name     string
		discover func(ctx context.Context, warnFn func(string)) ([]DiscoveredRepo, error)
	}

	var sources []sourceEntry
	for _, src := range cfg.Sources.Gitlab {
		for _, group := range src.Groups {
			src, group := src, group
			sources = append(sources, sourceEntry{
				name: src.Host + "/" + group,
				discover: func(ctx context.Context, warnFn func(string)) ([]DiscoveredRepo, error) {
					return ListGitlabGroupRepos(ctx, src.Host, group, src.TokenEnv, warnFn)
				},
			})
		}
	}
	for _, src := range cfg.Sources.Github {
		for _, org := range src.Orgs {
			src, org := src, org
			sources = append(sources, sourceEntry{
				name: src.Host + "/" + org,
				discover: func(ctx context.Context, warnFn func(string)) ([]DiscoveredRepo, error) {
					return ListGithubOrgRepos(ctx, src.Host, org, src.TokenEnv)
				},
			})
		}
	}

	if len(sources) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Phase 1: API calls — one TUI row per source group/org.
	sourceNames := make([]string, len(sources))
	for i, s := range sources {
		sourceNames[i] = s.name
	}

	var mu sync.Mutex
	var allRepos []DiscoveredRepo

	discoverView := newView(mode, sourceNames, nil, cancel)
	discoverTasks := make([]executor.Task, len(sources))
	for i, src := range sources {
		i, src := i, src
		discoverTasks[i] = executor.Task{
			Name: src.name,
			ExecuteFn: func(ctx context.Context, tName string, _ executor.OutputHandler) error {
				discoverView.SetDetail(tName, "discovering…")
				repos, err := src.discover(ctx, warnFn)
				if err != nil {
					return err
				}
				mu.Lock()
				allRepos = append(allRepos, repos...)
				mu.Unlock()
				noun := "repo"
				if len(repos) != 1 {
					noun = "repos"
				}
				discoverView.SetDetail(tName, fmt.Sprintf("found %d %s", len(repos), noun))
				return nil
			},
		}
	}

	discoverExec, err := executor.NewExecutor(discoverTasks, executor.ExecutorOptions{
		OnTaskStart:  discoverView.OnStart,
		OnTaskFinish: discoverView.OnFinish,
	})
	if err != nil {
		return err
	}

	discoverErr := discoverExec.Execute(ctx)
	discoverView.Wait()
	if discoverErr != nil {
		return discoverErr
	}

	if len(allRepos) == 0 {
		return nil
	}

	// Phase 2: Clone/pull — one TUI row per discovered repo.
	taskNames := make([]string, len(allRepos))
	dirs := make(map[string]string, len(allRepos))
	for i, r := range allRepos {
		taskNames[i] = r.Host + "/" + r.Path
		if localDir, err := config.LocalDirForRemote(r.Remote); err == nil {
			dirs[taskNames[i]] = localDir
		}
	}

	syncView := newView(mode, taskNames, dirs, cancel)
	syncTasks := make([]executor.Task, len(allRepos))
	for i, repo := range allRepos {
		i, repo := i, repo
		syncTasks[i] = executor.Task{
			Name: taskNames[i],
			ExecuteFn: func(ctx context.Context, tName string, _ executor.OutputHandler) error {
				setDetail := func(d string) { syncView.SetDetail(tName, d) }
				return syncTransientDetailed(ctx, cwd, repo.Remote, setDetail, warnFn)
			},
		}
	}

	syncExec, err := executor.NewExecutor(syncTasks, executor.ExecutorOptions{
		MaxParallelization: parallelLimit(8),
		OnTaskStart:        syncView.OnStart,
		OnTaskFinish:       syncView.OnFinish,
	})
	if err != nil {
		return err
	}

	runErr := syncExec.Execute(ctx)
	syncView.Wait()
	return runErr
}

// syncTransientDetailed clones or pulls a single transiently-discovered remote,
// reporting progress via setDetail.
func syncTransientDetailed(ctx context.Context, cwd, remote string, setDetail func(string), warnFn func(string)) error {
	localDir, err := config.LocalDirForRemote(remote)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, localDir)

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		setDetail("cloning…")
		if err := cloneRemote(ctx, cwd, remote, localDir); err != nil {
			return err
		}
		setDetail("cloned")
		return nil
	}

	if err := fetchOrigin(ctx, projectPath); err != nil {
		warnFn(fmt.Sprintf("fetch failed for %s: %v", localDir, err))
	}

	dirty, err := hasLocalChanges(ctx, projectPath)
	if err != nil {
		return err
	}
	if dirty {
		setDetail("skipped")
		return nil
	}

	branch := getCurrentBranch(ctx, projectPath)
	if branch == "" {
		return fmt.Errorf("cannot determine current branch in %s", projectPath)
	}

	upToDate, err := mergeFastForward(ctx, projectPath, "origin/"+branch)
	if err != nil {
		return err
	}
	if upToDate {
		setDetail("up to date")
	} else {
		setDetail("pulled")
	}
	return nil
}
