package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
// and clones/pulls them in parallel with a progress view.
// Discovered repos are transient (not written to config).
func Discover(ctx context.Context, cfg *config.GitteConfig, cwd string, mode output.OutputMode) error {
	var repos []DiscoveredRepo

	for _, src := range cfg.Sources.Gitlab {
		for _, group := range src.Groups {
			discovered, err := ListGitlabGroupRepos(ctx, src.Host, group, src.TokenEnv)
			if err != nil {
				return fmt.Errorf("gitlab discovery failed for group %s/%s: %w", src.Host, group, err)
			}
			repos = append(repos, discovered...)
		}
	}

	for _, src := range cfg.Sources.Github {
		for _, org := range src.Orgs {
			discovered, err := ListGithubOrgRepos(ctx, src.Host, org, src.TokenEnv)
			if err != nil {
				return fmt.Errorf("github discovery failed for org %s/%s: %w", src.Host, org, err)
			}
			repos = append(repos, discovered...)
		}
	}

	if len(repos) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	taskNames := make([]string, len(repos))
	dirs := make(map[string]string, len(repos))
	for i, r := range repos {
		taskNames[i] = r.Host + "/" + r.Path
		if localDir, err := config.LocalDirForRemote(r.Remote); err == nil {
			dirs[taskNames[i]] = localDir
		}
	}

	view := newView(mode, taskNames, dirs, cancel)

	tasks := make([]executor.Task, len(repos))
	for i, repo := range repos {
		i, repo := i, repo
		tasks[i] = executor.Task{
			Name: taskNames[i],
			ExecuteFn: func(ctx context.Context, tName string, _ executor.OutputHandler) error {
				setDetail := func(detail string) { view.SetDetail(tName, detail) }
				return syncTransientDetailed(ctx, cwd, repo.Remote, setDetail)
			},
		}
	}

	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{
		OnTaskStart:  view.OnStart,
		OnTaskFinish: view.OnFinish,
	})
	if err != nil {
		return err
	}

	runErr := exec.Execute(ctx)
	view.Wait()
	return runErr
}

// syncTransientDetailed clones or pulls a single transiently-discovered remote,
// reporting progress via setDetail.
func syncTransientDetailed(ctx context.Context, cwd, remote string, setDetail func(string)) error {
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
		fmt.Fprintf(os.Stderr, "warning: fetch failed for %s: %v\n", localDir, err)
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
