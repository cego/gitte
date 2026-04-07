package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var gitlabHTTPClient = &http.Client{Timeout: 30 * time.Second}

type gitlabProject struct {
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// ListGitlabGroupRepos returns all repos in a GitLab group (recursive, paginated)
func ListGitlabGroupRepos(ctx context.Context, host, group, tokenEnv string) ([]DiscoveredRepo, error) {
	token := os.Getenv(tokenEnv)
	if token == "" {
		fmt.Fprintf(os.Stderr, "warning: %s is not set — gitlab discovery for %s/%s may fail for private groups\n", tokenEnv, host, group)
	}

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

		projects, nextPage, err := fetchGitlabPage(req, group)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			repos = append(repos, DiscoveredRepo{
				Remote: p.SSHURLToRepo,
				Host:   host,
				Path:   p.PathWithNamespace,
			})
		}
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

func fetchGitlabPage(req *http.Request, group string) ([]gitlabProject, string, error) {
	resp, err := gitlabHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("gitlab API request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close gitlab response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		hint := ""
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			hint = " (invalid or expired token — check your token env var)"
		case http.StatusForbidden:
			hint = " (token lacks read_api scope or group access)"
		case http.StatusNotFound:
			hint = " (group not found or token missing — set the token env var and ensure it has read_api scope)"
		}
		return nil, "", fmt.Errorf("gitlab API returned %d for group %s%s", resp.StatusCode, group, hint)
	}

	var projects []gitlabProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, "", fmt.Errorf("failed to decode gitlab response: %w", err)
	}

	return projects, resp.Header.Get("X-Next-Page"), nil
}
