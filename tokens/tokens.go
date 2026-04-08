package tokens

import (
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

// Resolve returns the token to use for a source, following this priority chain:
//  1. token_cmd — run the shell command and use its trimmed stdout
//  2. token_env — read the named environment variable
//  3. system keyring — look up by kind and host
//
// Returns ("", nil) when no token is configured. The caller should warn the
// user when a token is needed but absent.
func Resolve(kind, host, tokenEnv, tokenCmd string) (string, error) {
	if tokenCmd != "" {
		out, err := exec.Command("sh", "-c", tokenCmd).Output()
		if err != nil {
			return "", fmt.Errorf("token_cmd for %s failed: %w", host, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	if tokenEnv != "" {
		return os.Getenv(tokenEnv), nil
	}
	token, err := Get(kind, host)
	if err != nil {
		// ErrNotFound or keyring unavailable — proceed without token
		return "", nil
	}
	return token, nil
}
