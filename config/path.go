package config

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	sshRe   = regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)
	httpsRe = regexp.MustCompile(`^https?://([^/]+)/(.+?)(?:\.git)?$`)
)

// ParseRemoteURL parses a git remote URL and returns (host, namespacedPath, localDir)
// Supports:
//
//	git@gitlab.example.com:org/services/myrepo.git  -> gitlab.example.com/org/services/myrepo
//	https://github.com/example/myrepo.git           -> github.com/example/myrepo
func ParseRemoteURL(remote string) (host, path, localDir string, err error) {
	remote = strings.TrimSpace(remote)

	// SSH format: git@host:path.git
	if m := sshRe.FindStringSubmatch(remote); m != nil {
		host = m[1]
		path = m[2]
		localDir = host + "/" + path
		return
	}

	// HTTPS format: https://host/path.git
	if m := httpsRe.FindStringSubmatch(remote); m != nil {
		host = m[1]
		path = m[2]
		localDir = host + "/" + path
		return
	}

	err = fmt.Errorf("unsupported remote URL format: %q", remote)
	return
}

// LocalDirForRemote returns the local directory path for a project's remote URL
func LocalDirForRemote(remote string) (string, error) {
	_, _, localDir, err := ParseRemoteURL(remote)
	return localDir, err
}
