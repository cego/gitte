package config

import (
	"fmt"
	"path/filepath"
)

// FilterProjectsByGlob returns a shallow copy of cfg with Projects filtered.
// If includes is non-empty, only projects matching at least one pattern are kept.
// Projects matching any exclude pattern are always removed.
// Uses filepath.Match semantics (*, ?, [...] are supported; ** is not).
func FilterProjectsByGlob(cfg *GitteConfig, includes, excludes []string) (*GitteConfig, error) {
	for _, p := range append(includes, excludes...) {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("invalid filter pattern %q: %w", p, err)
		}
	}

	filtered := make(map[string]ProjectConfig, len(cfg.Projects))
	for name, proj := range cfg.Projects {
		if len(includes) > 0 {
			matched := false
			for _, p := range includes {
				if ok, _ := filepath.Match(p, name); ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		excluded := false
		for _, p := range excludes {
			if ok, _ := filepath.Match(p, name); ok {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		filtered[name] = proj
	}

	out := *cfg
	out.Projects = filtered
	return &out, nil
}
