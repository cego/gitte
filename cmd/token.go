package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/tokens"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage API tokens in the system keyring",
		Long: `Store and retrieve API tokens for GitLab and GitHub hosts.

Tokens are stored in the system keyring (macOS Keychain or GNOME Keyring on Linux)
and looked up automatically during 'gitte gitops --discover'.

One token is stored per host, so multiple GitLab instances each have their own entry.

If the system keyring is unavailable (e.g. headless servers), use token_env or
token_cmd in your source config instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTokenSetCmd(),
		newTokenGetCmd(),
		newTokenDeleteCmd(),
		newTokenListCmd(),
	)
	return cmd
}

func newTokenSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <gitlab|github> <host>",
		Short: "Store an API token in the system keyring",
		Long: `Store an API token for a GitLab or GitHub host in the system keyring.

The token is looked up automatically when running 'gitte gitops --discover'.
If stdin is a terminal the input is hidden; otherwise the token is read from stdin.

Examples:
  gitte token set gitlab gitlab.example.com
  gitte token set github github.com
  echo "$TOKEN" | gitte token set gitlab gitlab.example.com`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := args[0]
			host := args[1]
			if kind != "gitlab" && kind != "github" {
				return fmt.Errorf("kind must be 'gitlab' or 'github', got %q", kind)
			}

			var token string
			if term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintf(os.Stderr, "Enter token for %s (input hidden): ", host)
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return fmt.Errorf("reading token: %w", err)
				}
				token = strings.TrimSpace(string(b))
			} else {
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					token = strings.TrimSpace(scanner.Text())
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("reading token from stdin: %w", err)
				}
			}

			if token == "" {
				return fmt.Errorf("token must not be empty")
			}

			if err := tokens.Set(kind, host, token); err != nil {
				return fmt.Errorf("failed to store token: %w\n\nIf the system keyring is unavailable, use --token-env when running 'gitte sources add'", err)
			}

			fmt.Printf("Token stored for %s (%s)\n", host, kind)
			return nil
		},
	}
}

func newTokenGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <gitlab|github> <host>",
		Short: "Retrieve and print a stored token (for diagnostics)",
		Long: `Print the token stored in the keyring for a given host.
Useful for diagnosing whether the keyring is accessible and the token is present.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := args[0]
			host := args[1]
			if kind != "gitlab" && kind != "github" {
				return fmt.Errorf("kind must be 'gitlab' or 'github', got %q", kind)
			}
			token, err := tokens.Get(kind, host)
			if err != nil {
				return fmt.Errorf("failed to retrieve token for %s: %w", host, err)
			}
			fmt.Println(token)
			return nil
		},
	}
}

func newTokenDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <gitlab|github> <host>",
		Short: "Remove an API token from the system keyring",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := args[0]
			host := args[1]
			if kind != "gitlab" && kind != "github" {
				return fmt.Errorf("kind must be 'gitlab' or 'github', got %q", kind)
			}
			if err := tokens.Delete(kind, host); err != nil {
				return fmt.Errorf("failed to delete token: %w", err)
			}
			fmt.Printf("Token removed for %s (%s)\n", host, kind)
			return nil
		},
	}
}

func newTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show keyring token status for configured sources",
		Long: `Show which configured sources use the keyring and whether a token is stored.

Sources using token_env or token_cmd are not shown since they do not use the keyring.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			override, err := config.LoadOverrideConfig(globalCwd)
			if err != nil {
				return fmt.Errorf("failed to load local override: %w", err)
			}

			found := false
			for _, src := range override.Sources.Gitlab {
				if src.TokenCmd != "" || src.TokenEnv != "" {
					continue
				}
				token, err := tokens.Get("gitlab", src.Host)
				status := "stored"
				if err != nil || token == "" {
					status = "not set — run: gitte token set gitlab " + src.Host
				}
				fmt.Printf("gitlab  %s  [%s]\n", src.Host, status)
				found = true
			}
			for _, src := range override.Sources.Github {
				if src.TokenCmd != "" || src.TokenEnv != "" {
					continue
				}
				token, err := tokens.Get("github", src.Host)
				status := "stored"
				if err != nil || token == "" {
					status = "not set — run: gitte token set github " + src.Host
				}
				fmt.Printf("github  %s  [%s]\n", src.Host, status)
				found = true
			}
			if !found {
				fmt.Println("No sources are configured to use the keyring.")
				fmt.Println("Sources using token_env or token_cmd are excluded from this list.")
			}
			return nil
		},
	}
}
