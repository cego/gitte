// Package doctor implements the gitte doctor diagnostic command.
package doctor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/tokens"
)

// Result holds the outcome of a single diagnostic check.
type Result struct {
	Name   string
	Pass   bool
	Output string // combined stdout+stderr, may be empty
	Hint   string // optional fix hint shown on failure
}

// Run executes all built-in and config-driven doctor checks and returns the results.
func Run(ctx context.Context, cfg *config.GitteConfig, cwd string) []Result {
	var results []Result

	// ── Built-in checks ──────────────────────────────────────────────────
	results = append(results, checkGitConfig(ctx))
	results = append(results, checkSSH(ctx, cfg)...)
	results = append(results, checkTokens(cfg)...)
	results = append(results, checkProjectDirs(cfg, cwd)...)
	results = append(results, checkStartupChecks(ctx, cfg, cwd)...)

	// ── Config-driven checks ─────────────────────────────────────────────
	for name, dc := range cfg.DoctorChecks {
		dc := dc
		output, pass := dc.Run(ctx, cwd)
		hint := dc.Hint
		results = append(results, Result{
			Name:   name,
			Pass:   pass,
			Output: output,
			Hint:   hint,
		})
	}

	return results
}

// Print writes a formatted doctor report to stdout.
// Returns true if all checks passed.
func Print(results []Result) bool {
	allPass := true
	for _, r := range results {
		if !r.Pass {
			allPass = false
		}
	}

	for _, r := range results {
		status := "✓"
		if !r.Pass {
			status = "✗"
		}
		fmt.Printf("%s %s\n", status, r.Name)
		if r.Output != "" {
			for _, line := range strings.Split(r.Output, "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
		if !r.Pass && r.Hint != "" {
			fmt.Printf("    hint: %s\n", r.Hint)
		}
	}
	return allPass
}

// checkGitConfig verifies that git user.name and user.email are configured.
func checkGitConfig(ctx context.Context) Result {
	var lines []string
	pass := true

	name := runGitConfig(ctx, "user.name")
	if name == "" {
		pass = false
		lines = append(lines, "user.name is not set")
	} else {
		lines = append(lines, "user.name = "+name)
	}

	email := runGitConfig(ctx, "user.email")
	if email == "" {
		pass = false
		lines = append(lines, "user.email is not set")
	} else {
		lines = append(lines, "user.email = "+email)
	}

	return Result{
		Name:   "git config",
		Pass:   pass,
		Output: strings.Join(lines, "\n"),
		Hint:   "run: git config --global user.name \"Your Name\" && git config --global user.email you@example.com",
	}
}

func runGitConfig(ctx context.Context, key string) string {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "config", "--global", key) //nolint:gosec
	cmd.Stdout = &out
	_ = cmd.Run()
	return strings.TrimSpace(out.String())
}

// checkSSH tests SSH connectivity to each configured source host.
func checkSSH(ctx context.Context, cfg *config.GitteConfig) []Result {
	hosts := collectSSHHosts(cfg)
	var results []Result
	for _, host := range hosts {
		output, pass := testSSH(ctx, host)
		results = append(results, Result{
			Name:   "ssh " + host,
			Pass:   pass,
			Output: output,
			Hint:   fmt.Sprintf("check your SSH key is added to %s and loaded in your SSH agent", host),
		})
	}
	return results
}

func collectSSHHosts(cfg *config.GitteConfig) []string {
	seen := map[string]bool{}
	var hosts []string
	for _, src := range cfg.Sources.Gitlab {
		if !seen[src.Host] {
			seen[src.Host] = true
			hosts = append(hosts, src.Host)
		}
	}
	for _, src := range cfg.Sources.Github {
		if !seen[src.Host] {
			seen[src.Host] = true
			hosts = append(hosts, src.Host)
		}
	}
	return hosts
}

func testSSH(ctx context.Context, host string) (output string, pass bool) {
	var buf bytes.Buffer
	// ssh -T exits 1 on GitHub/GitLab (no shell access) but prints a welcome message.
	// We treat any response as success; failure to connect is the error case.
	cmd := exec.CommandContext(ctx, "ssh", "-T", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10", "git@"+host) //nolint:gosec
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		// ssh -T to GitHub/GitLab returns exit code 1 even on success.
		// Distinguish by checking for known success strings.
		if strings.Contains(out, "successfully authenticated") ||
			strings.Contains(out, "Welcome to GitLab") ||
			strings.Contains(out, "Hi ") {
			return out, true
		}
		return out, false
	}
	return out, true
}

// checkTokens verifies that API tokens are configured for each source.
func checkTokens(cfg *config.GitteConfig) []Result {
	var results []Result
	for _, src := range cfg.Sources.Gitlab {
		token, err := tokens.Resolve("gitlab", src.Host, src.TokenEnv, src.TokenCmd)
		pass := err == nil && token != ""
		output := ""
		if pass {
			output = "token configured"
		} else if err != nil {
			output = "error: " + err.Error()
		}
		hint := ""
		if !pass {
			hint = fmt.Sprintf("run: gitte token set gitlab %s", src.Host)
		}
		results = append(results, Result{
			Name:   "token gitlab/" + src.Host,
			Pass:   pass,
			Output: output,
			Hint:   hint,
		})
	}
	for _, src := range cfg.Sources.Github {
		token, err := tokens.Resolve("github", src.Host, src.TokenEnv, src.TokenCmd)
		pass := err == nil && token != ""
		output := ""
		if pass {
			output = "token configured"
		} else if err != nil {
			output = "error: " + err.Error()
		}
		hint := ""
		if !pass {
			hint = fmt.Sprintf("run: gitte token set github %s", src.Host)
		}
		results = append(results, Result{
			Name:   "token github/" + src.Host,
			Pass:   pass,
			Output: output,
			Hint:   hint,
		})
	}
	return results
}

// checkProjectDirs verifies that configured project directories exist.
func checkProjectDirs(cfg *config.GitteConfig, cwd string) []Result {
	missing := 0
	total := len(cfg.Projects)
	var lines []string
	for name, proj := range cfg.Projects {
		localDir, err := config.LocalDirForRemote(proj.Remote)
		if err != nil {
			continue
		}
		p := filepath.Join(cwd, localDir)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			missing++
			lines = append(lines, fmt.Sprintf("missing: %s", name))
		}
	}
	pass := missing == 0
	output := fmt.Sprintf("%d/%d directories present", total-missing, total)
	if len(lines) > 0 && len(lines) <= 10 {
		output += "\n" + strings.Join(lines, "\n")
	} else if len(lines) > 10 {
		output += fmt.Sprintf("\n(%d missing repos, run 'gitte gitops' to clone them)", missing)
	}
	return []Result{{
		Name:   "project directories",
		Pass:   pass,
		Output: output,
		Hint:   "run: gitte gitops",
	}}
}

// checkStartupChecks runs each configured startup check and reports pass/fail.
func checkStartupChecks(ctx context.Context, cfg *config.GitteConfig, cwd string) []Result {
	var results []Result
	for name, sc := range cfg.StartupChecks {
		err := sc.Check(ctx, cwd)
		pass := err == nil
		output := ""
		if err != nil {
			output = err.Error()
		}
		hint := sc.GetHint()
		results = append(results, Result{
			Name:   "startup: " + name,
			Pass:   pass,
			Output: output,
			Hint:   hint,
		})
	}
	return results
}
