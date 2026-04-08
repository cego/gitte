package gitops

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/cego/gitte/config"
)

// RepoStatus holds the local state of a single project directory.
type RepoStatus struct {
	Name          string
	LocalDir      string // relative path under cwd
	Dirty         bool   // has uncommitted changes
	AheadCount    int    // commits ahead of origin/defaultBranch
	Detached      bool   // detached HEAD
	Branch        string // current branch (empty if detached)
	DefaultBranch string
	Missing       bool // directory does not exist yet
}

// Scan returns the local status of all configured projects without contacting
// any remote. It is intentionally fast: no network calls are made.
func Scan(ctx context.Context, cfg *config.GitteConfig, cwd string) ([]RepoStatus, error) {
	names := make([]string, 0, len(cfg.Projects))
	for name := range cfg.Projects {
		names = append(names, name)
	}
	sort.Strings(names)

	var results []RepoStatus
	for _, name := range names {
		proj := cfg.Projects[name]
		localDir, err := config.LocalDirForRemote(proj.Remote)
		if err != nil {
			continue
		}
		projectPath := filepath.Join(cwd, localDir)

		defaultBranch := proj.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "master"
		}

		st := RepoStatus{
			Name:          name,
			LocalDir:      localDir,
			DefaultBranch: defaultBranch,
		}

		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			st.Missing = true
			results = append(results, st)
			continue
		}

		detached, err := isDetachedHEAD(ctx, projectPath)
		if err == nil && detached {
			st.Detached = true
		} else {
			st.Branch = getCurrentBranch(ctx, projectPath)
		}

		dirty, _ := hasLocalChanges(ctx, projectPath)
		st.Dirty = dirty

		// Count commits ahead of origin/defaultBranch (local-only, no fetch).
		st.AheadCount = commitsAhead(ctx, projectPath, "origin/"+defaultBranch, "HEAD")

		results = append(results, st)
	}
	return results, nil
}
