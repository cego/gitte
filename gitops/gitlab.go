package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var gitlabHTTPClient = &http.Client{Timeout: 30 * time.Second}

type gitlabProject struct {
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// ListGitlabGroupRepos returns all repos in a GitLab group (recursive, paginated).
// token is the resolved API token; pass "" for unauthenticated (public groups only).
func ListGitlabGroupRepos(ctx context.Context, host, group, token string, warnFn func(string)) ([]DiscoveredRepo, error) {
	if token == "" {
		warnFn(fmt.Sprintf("no token for %s — private groups may be inaccessible (run: gitte token set gitlab %s)", host, host))
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

		projects, nextPage, err := fetchGitlabPage(req, host, group)
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

func fetchGitlabPage(req *http.Request, host, group string) ([]gitlabProject, string, error) {
	resp, err := gitlabHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("gitlab API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		hint := ""
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			hint = fmt.Sprintf(" (invalid or expired token — run: gitte token set gitlab %s)", host)
		case http.StatusForbidden:
			hint = " (token lacks read_api scope or group access)"
		case http.StatusNotFound:
			hint = fmt.Sprintf(" (group not found or no token — run: gitte token set gitlab %s)", host)
		}
		return nil, "", fmt.Errorf("gitlab API returned %d for group %s%s", resp.StatusCode, group, hint)
	}

	var projects []gitlabProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, "", fmt.Errorf("failed to decode gitlab response: %w", err)
	}

	return projects, resp.Header.Get("X-Next-Page"), nil
}
