package tokens

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

const service = "gitte"

func account(kind, host string) string {
	return kind + ":" + host
}

// Get retrieves the stored token for the given provider kind ("gitlab"/"github") and host.
// Returns keyring.ErrNotFound if no token is stored.
func Get(kind, host string) (string, error) {
	return keyring.Get(service, account(kind, host))
}

// Set stores a token for the given provider kind and host in the system keyring.
func Set(kind, host, token string) error {
	return keyring.Set(service, account(kind, host), token)
}

// Delete removes the stored token for the given provider kind and host.
func Delete(kind, host string) error {
	return keyring.Delete(service, account(kind, host))
}

// Resolve returns the token to use for a source. The keyring is always tried
// first. If it has no entry or is unavailable, falls back to token_cmd then
// token_env. Returns ("", nil) when no token is found anywhere.
func Resolve(kind, host, tokenEnv, tokenCmd string) (string, error) {
	if token, err := Get(kind, host); err == nil && token != "" {
		return token, nil
	}
	if tokenCmd != "" {
		out, err := exec.CommandContext(context.Background(), "sh", "-c", tokenCmd).Output()
		if err != nil {
			return "", fmt.Errorf("token_cmd for %s failed: %w", host, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	if tokenEnv != "" {
		return os.Getenv(tokenEnv), nil
	}
	return "", nil
}
