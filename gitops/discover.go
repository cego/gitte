package gitops

import (
	"context"
	"fmt"
	"os"

	"github.com/cego/gitte/config"
)

// DiscoveredRepo represents a repo found via API discovery
type DiscoveredRepo struct {
	Remote string
	Host   string
	Path   string
}

// Discover fetches all repos from configured sources (GitLab groups + GitHub orgs)
// and clones/pulls them. Discovered repos are transient (not written to config).
func Discover(ctx context.Context, cfg *config.GitteConfig, cwd string) error {
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

	for _, repo := range repos {
		if err := SyncTransient(ctx, repo.Remote, cwd); err != nil {
			fmt.Fprintf(os.Stderr, "warning: discovery sync failed for %s: %v\n", repo.Remote, err)
		}
	}

	return nil
}
