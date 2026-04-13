package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var githubHTTPClient = &http.Client{Timeout: 30 * time.Second}

type githubRepo struct {
	SSHURL   string `json:"ssh_url"`
	FullName string `json:"full_name"`
}

// ListGithubOrgRepos returns all repos in a GitHub org (paginated).
// token is the resolved API token; pass "" for unauthenticated (public orgs only).
func ListGithubOrgRepos(ctx context.Context, host, org, token string) ([]DiscoveredRepo, error) {
	var repos []DiscoveredRepo
	page := 1

	for {
		apiURL := fmt.Sprintf("https://api.%s/orgs/%s/repos?per_page=100&page=%d", host, org, page)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		ghRepos, err := fetchGithubPage(req, host, org)
		if err != nil {
			return nil, err
		}
		if len(ghRepos) == 0 {
			break
		}
		for _, r := range ghRepos {
			repos = append(repos, DiscoveredRepo{
				Remote: r.SSHURL,
				Host:   host,
				Path:   r.FullName,
			})
		}
		page++
	}

	return repos, nil
}

func fetchGithubPage(req *http.Request, host, org string) ([]githubRepo, error) {
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		hint := ""
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			hint = fmt.Sprintf(" (invalid or expired token — run: gitte token set github %s)", host)
		case http.StatusForbidden:
			hint = " (token lacks read:org scope or org access)"
		case http.StatusNotFound:
			hint = fmt.Sprintf(" (org not found or no token — run: gitte token set github %s)", host)
		}
		return nil, fmt.Errorf("github API returned %d for org %s%s", resp.StatusCode, org, hint)
	}

	var ghRepos []githubRepo
	if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
		return nil, fmt.Errorf("failed to decode github response: %w", err)
	}
	return ghRepos, nil
}
