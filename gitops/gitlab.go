package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type gitlabProject struct {
	SSHURLToRepo string `json:"ssh_url_to_repo"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// ListGitlabGroupRepos returns all repos in a GitLab group (recursive, paginated)
func ListGitlabGroupRepos(ctx context.Context, host, group, tokenEnv string) ([]DiscoveredRepo, error) {
	token := os.Getenv(tokenEnv)

	var repos []DiscoveredRepo
	page := 1
	encodedGroup := url.PathEscape(group)

	for {
		apiURL := fmt.Sprintf("https://%s/api/v4/groups/%s/projects?include_subgroups=true&per_page=100&page=%d",
			host, encodedGroup, page)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("PRIVATE-TOKEN", token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gitlab API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("gitlab API returned %d for group %s", resp.StatusCode, group)
		}

		var projects []gitlabProject
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			return nil, fmt.Errorf("failed to decode gitlab response: %w", err)
		}

		for _, p := range projects {
			repos = append(repos, DiscoveredRepo{
				Remote: p.SSHURLToRepo,
				Host:   host,
				Path:   p.PathWithNamespace,
			})
		}

		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" {
			break
		}
		next, err := strconv.Atoi(nextPage)
		if err != nil || next <= page {
			break
		}
		page = next
	}

	return repos, nil
}
