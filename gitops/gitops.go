package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitte/config"
	"gitte/executor"
	"gitte/output"
)

// CheckoutPrompt is raised for each project that the user may want to act on
// (detached HEAD, broken remote, stale branch). In TTY mode the user is asked
// interactively; in plain mode the command fails with Recommendation as a hint.
type CheckoutPrompt struct {
	ProjectName    string
	ProjectPath    string // absolute path to the local checkout
	DefaultBranch  string
	Reason         string       // human-readable description of the problem
	Recommendation string       // git command(s) that resolve the problem
	retryFn        func() error // re-syncs the project after a successful checkout
}

// Sync clones or pulls all projects. In TTY mode a live progress TUI is shown.
// onPrompt is called serially after the TUI exits for each project that needs
// attention; it returns (true, nil) to check out the default branch, (false, nil)
// to skip, or (false, err) to surface an error (used by plain mode).
func Sync(
	ctx context.Context,
	cfg *config.GitteConfig,
	cwd string,
	mode output.OutputMode,
	noRebase bool,
	onPrompt func(CheckoutPrompt) (bool, error),
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	projectNames := make([]string, 0, len(cfg.Projects))
	for name := range cfg.Projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	taskNames := make([]string, len(projectNames))
	for i, n := range projectNames {
		taskNames[i] = "gitops:" + n
	}

	view := newView(mode, taskNames, cancel)

	var mu sync.Mutex
	var prompts []CheckoutPrompt

	addPrompt := func(p CheckoutPrompt) {
		mu.Lock()
		prompts = append(prompts, p)
		mu.Unlock()
	}

	tasks := make([]executor.Task, 0, len(projectNames))
	for _, name := range projectNames {
		name := name
		proj := cfg.Projects[name]
		tasks = append(tasks, executor.Task{
			Name: "gitops:" + name,
			ExecuteFn: func(ctx context.Context, taskName string, handler executor.OutputHandler) error {
				setDetail := func(detail string) { view.SetDetail(taskName, detail) }
				return syncProject(ctx, cwd, name, proj, noRebase, setDetail, addPrompt)
			},
		})
	}

	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{
		OnTaskStart:  view.OnStart,
		OnTaskFinish: view.OnFinish,
	})
	if err != nil {
		return err
	}

	runErr := exec.Execute(ctx)
	view.Wait()

	// Process post-TUI prompts serially.
	var promptErrs []error
	if onPrompt != nil {
		for _, p := range prompts {
			doCheckout, err := onPrompt(p)
			if err != nil {
				promptErrs = append(promptErrs, err)
				continue
			}
			if doCheckout {
				if err := checkoutBranch(p.ProjectPath, p.DefaultBranch); err != nil {
					promptErrs = append(promptErrs, fmt.Errorf("[%s] checkout failed: %w", p.ProjectName, err))
				} else if p.retryFn != nil {
					if err := p.retryFn(); err != nil {
						promptErrs = append(promptErrs, fmt.Errorf("[%s] sync after checkout: %w", p.ProjectName, err))
					}
				}
			}
		}
	}

	return errors.Join(append(promptErrs, runErr)...)
}

// SyncTransient clones or pulls a single transiently-discovered remote.
func SyncTransient(ctx context.Context, remote, cwd string) error {
	localDir, err := config.LocalDirForRemote(remote)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, localDir)

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return cloneRemote(cwd, remote, localDir)
	}

	if err := fetchOrigin(ctx, projectPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: fetch failed for %s: %v\n", localDir, err)
	}

	dirty, err := hasLocalChanges(projectPath)
	if err != nil {
		return err
	}
	if dirty {
		fmt.Printf("warning: skipping pull for %s (local changes present)\n", localDir)
		return nil
	}

	branch := getCurrentBranch(projectPath)
	if branch == "" {
		return fmt.Errorf("cannot determine current branch in %s", projectPath)
	}

	_, err = mergeFastForward(projectPath, "origin/"+branch)
	return err
}

// syncProject performs the full clone/fetch/merge/rebase flow for one project.
func syncProject(
	ctx context.Context,
	cwd, name string,
	proj config.ProjectConfig,
	noRebase bool,
	setDetail func(string),
	addPrompt func(CheckoutPrompt),
) error {
	localDir, err := config.LocalDirForRemote(proj.Remote)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(cwd, localDir)

	defaultBranch := proj.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "master"
	}

	// retryFn re-syncs the project after a successful checkout.
	// The TUI is already gone by the time this runs, so we print plain output.
	retryFn := func() error {
		return syncProject(ctx, cwd, name, proj, noRebase,
			func(d string) { fmt.Printf("  [%s] %s\n", name, d) },
			func(_ CheckoutPrompt) {}, // no further prompts after retry
		)
	}

	// Clone if directory does not exist yet.
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		setDetail("cloning…")
		if err := cloneRemote(cwd, proj.Remote, localDir); err != nil {
			return err
		}
		setDetail("cloned")
		return nil
	}

	// Always fetch so remote refs are fresh.
	if err := fetchOrigin(ctx, projectPath); err != nil {
		// Non-fatal: continue with cached refs; subsequent checks may be stale.
		fmt.Fprintf(os.Stderr, "warning [%s]: fetch failed: %v\n", name, err)
	}

	// Detached HEAD.
	if detached, _ := isDetachedHEAD(projectPath); detached {
		setDetail("detached: not on a branch")
		addPrompt(CheckoutPrompt{
			ProjectName:    name,
			ProjectPath:    projectPath,
			DefaultBranch:  defaultBranch,
			Reason:         "not on a branch (detached HEAD)",
			Recommendation: fmt.Sprintf("git -C %s checkout %s", projectPath, defaultBranch),
			retryFn:        retryFn,
		})
		return nil
	}

	currentBranch := getCurrentBranch(projectPath)

	// Local changes: skip pull/rebase.
	dirty, err := hasLocalChanges(projectPath)
	if err != nil {
		return err
	}
	if dirty {
		setDetail("skipped")
		if currentBranch != defaultBranch {
			addStaleIfNeeded(name, projectPath, defaultBranch, retryFn, setDetail, addPrompt)
		}
		return nil
	}

	// ── On default branch ──────────────────────────────────────────────────
	if currentBranch == defaultBranch {
		upToDate, err := mergeFastForward(projectPath, "origin/"+defaultBranch)
		if err != nil {
			return err
		}
		if upToDate {
			setDetail("up to date")
		} else {
			setDetail("pulled")
		}
		return nil
	}

	// ── On non-default branch ───────────────────────────────────────────────
	remoteCurrentRef := "origin/" + currentBranch

	// Broken remote: tracking branch configured but no longer exists on origin.
	if !remoteRefExists(projectPath, remoteCurrentRef) && hasRemoteTrackingConfig(projectPath, currentBranch) {
		reason := fmt.Sprintf("remote branch '%s' no longer exists", currentBranch)
		setDetail("detached: " + reason)
		addPrompt(CheckoutPrompt{
			ProjectName:    name,
			ProjectPath:    projectPath,
			DefaultBranch:  defaultBranch,
			Reason:         reason,
			Recommendation: fmt.Sprintf("git -C %s checkout %s", projectPath, defaultBranch),
			retryFn:        retryFn,
		})
		return nil
	}

	// Pull from remote tracking branch if it has new commits.
	pulledLabel := ""
	if remoteRefExists(projectPath, remoteCurrentRef) {
		ahead := commitsAhead(projectPath, "HEAD", remoteCurrentRef)
		if ahead > 0 {
			if _, err := mergeFastForward(projectPath, remoteCurrentRef); err != nil {
				// Diverged from own remote — warn but don't fail.
				setDetail(fmt.Sprintf("stale: diverged from origin/%s", currentBranch))
			} else {
				pulledLabel = fmt.Sprintf("pulled %d from origin/%s", ahead, currentBranch)
			}
		}
	}

	// Auto-rebase onto default branch (unless disabled).
	if !noRebase {
		remoteDefaultRef := "origin/" + defaultBranch
		if commitsAhead(projectPath, "HEAD", remoteDefaultRef) > 0 {
			ok, err := tryRebase(projectPath, remoteDefaultRef)
			if err != nil {
				return err
			}
			if ok {
				if pulledLabel != "" {
					setDetail(pulledLabel + ", rebased onto " + defaultBranch)
				} else {
					setDetail("rebased onto " + defaultBranch)
				}
				return nil
			}
			// Rebase had conflicts and was aborted.  Fall through to stale check.
			setDetail(fmt.Sprintf("stale: rebase conflicts with %s", defaultBranch))
			addPrompt(CheckoutPrompt{
				ProjectName:    name,
				ProjectPath:    projectPath,
				DefaultBranch:  defaultBranch,
				Reason:         fmt.Sprintf("rebase conflicts with %s (has local work)", defaultBranch),
				Recommendation: fmt.Sprintf("git -C %s rebase origin/%s", projectPath, defaultBranch),
				retryFn:        retryFn,
			})
			return nil
		}
	}

	// Stale check for projects not yet brought up to date.
	if !addStaleIfNeeded(name, projectPath, defaultBranch, retryFn, setDetail, addPrompt) {
		if pulledLabel != "" {
			setDetail(pulledLabel)
		} else {
			setDetail("up to date")
		}
	}

	return nil
}

// addStaleIfNeeded checks whether the project is behind origin/<defaultBranch>
// by more than one week.  If so it updates the TUI detail and registers a
// checkout prompt.  Returns true when a prompt was added.
func addStaleIfNeeded(name, dir, defaultBranch string, retryFn func() error, setDetail func(string), addPrompt func(CheckoutPrompt)) bool {
	days := staleDays(dir, defaultBranch)
	if days == 0 {
		return false
	}

	hasWork := commitsAhead(dir, "origin/"+defaultBranch, "HEAD") > 0

	reason := fmt.Sprintf("%d days behind %s", days, defaultBranch)
	rec := fmt.Sprintf("git -C %s checkout %s", dir, defaultBranch)
	if hasWork {
		reason += " (has local work)"
		rec = fmt.Sprintf("git -C %s rebase origin/%s", dir, defaultBranch)
	}

	setDetail("stale: " + reason)
	addPrompt(CheckoutPrompt{
		ProjectName:    name,
		ProjectPath:    dir,
		DefaultBranch:  defaultBranch,
		Reason:         reason,
		Recommendation: rec,
		retryFn:        retryFn,
	})
	return true
}

// staleDays returns how many days behind origin/<defaultBranch> the current
// branch is, measured at the newest unreachable commit.  Returns 0 when on
// the default branch, already up-to-date, or when the check cannot run.
func staleDays(dir, defaultBranch string) int {
	if defaultBranch == "" {
		defaultBranch = "master"
	}
	res, err := executor.ExecuteSyncInDir(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return 0
	}
	if b := strings.TrimSpace(string(res.Stdout)); b == "HEAD" || b == defaultBranch {
		return 0
	}

	remoteRef := "origin/" + defaultBranch
	res2, err := executor.ExecuteSyncInDir(dir, "git", "log", "HEAD.."+remoteRef, "--format=%ct", "--max-count=1")
	if err != nil || res2.ExitCode != 0 {
		return 0
	}
	tsStr := strings.TrimSpace(string(res2.Stdout))
	if tsStr == "" {
		return 0
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0
	}

	days := int(time.Since(time.Unix(ts, 0)).Hours() / 24)
	if days > 7 {
		return days
	}
	return 0
}

// ── git helpers ──────────────────────────────────────────────────────────────

const fetchTimeout = 60 * time.Second

func fetchOrigin(ctx context.Context, dir string) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(fetchCtx, "git", "fetch", "origin") //nolint:gosec
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if fetchCtx.Err() != nil {
			return fmt.Errorf("git fetch timed out after %s", fetchTimeout)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("git fetch failed (exit %d): %s", exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

func getCurrentBranch(dir string) string {
	res, err := executor.ExecuteSyncInDir(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(string(res.Stdout))
}

func remoteRefExists(dir, ref string) bool {
	res, err := executor.ExecuteSyncInDir(dir, "git", "rev-parse", "--verify", ref)
	return err == nil && res.ExitCode == 0
}

func hasRemoteTrackingConfig(dir, branch string) bool {
	res, err := executor.ExecuteSyncInDir(dir, "git", "config", "--get", "branch."+branch+".remote")
	return err == nil && res.ExitCode == 0 && len(strings.TrimSpace(string(res.Stdout))) > 0
}

// commitsAhead returns the number of commits reachable from target but not from base.
func commitsAhead(dir, base, target string) int {
	res, err := executor.ExecuteSyncInDir(dir, "git", "rev-list", "--count", base+".."+target)
	if err != nil || res.ExitCode != 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(res.Stdout)))
	if err != nil {
		return 0
	}
	return n
}

func isDetachedHEAD(dir string) (bool, error) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return false, fmt.Errorf("git rev-parse: %w", err)
	}
	if res.ExitCode != 0 {
		return false, nil
	}
	return strings.TrimSpace(string(res.Stdout)) == "HEAD", nil
}

func hasLocalChanges(dir string) (bool, error) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, fmt.Errorf("git status failed (exit %d)", res.ExitCode)
	}
	return len(strings.TrimSpace(string(res.Stdout))) > 0, nil
}

func cloneRemote(cwd, remote, localDir string) error {
	res, err := executor.ExecuteSyncInDir(cwd, "git", "clone", remote, localDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	if res.ExitCode != 0 {
		if regexp.MustCompile(`(?i)permission denied`).Match(res.Stderr) {
			return fmt.Errorf("permission denied cloning %q: check SSH keys", remote)
		}
		return fmt.Errorf("git clone failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

// mergeFastForward runs git merge --ff-only <ref> and returns whether HEAD was
// already up-to-date.
func mergeFastForward(dir, ref string) (upToDate bool, err error) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "merge", "--ff-only", ref)
	if err != nil {
		return false, fmt.Errorf("git merge: %w", err)
	}
	if res.ExitCode != 0 {
		stderr := string(res.Stderr)
		if regexp.MustCompile(`(?i)(not possible to fast.forward|cannot fast.forward|needs merge)`).MatchString(stderr) {
			return false, fmt.Errorf("cannot fast-forward; branches have diverged")
		}
		return false, fmt.Errorf("git merge --ff-only failed (exit %d): %s", res.ExitCode, strings.TrimSpace(stderr))
	}
	return strings.Contains(string(res.Stdout), "Already up to date."), nil
}

// tryRebase attempts git rebase <onto>.  On conflict it aborts the rebase and
// returns (false, nil).  Returns (true, nil) on success.
func tryRebase(dir, onto string) (bool, error) {
	res, err := executor.ExecuteSyncInDir(dir, "git", "rebase", onto)
	if err != nil {
		return false, fmt.Errorf("git rebase: %w", err)
	}
	if res.ExitCode == 0 {
		return true, nil
	}
	// Rebase failed — abort to restore pre-rebase state.
	abort, abortErr := executor.ExecuteSyncInDir(dir, "git", "rebase", "--abort")
	if abortErr != nil {
		return false, fmt.Errorf("rebase failed and abort also failed: %w", abortErr)
	}
	if abort.ExitCode != 0 {
		return false, fmt.Errorf("rebase abort failed (exit %d): %s", abort.ExitCode, strings.TrimSpace(string(abort.Stderr)))
	}
	return false, nil
}

func checkoutBranch(dir, branch string) error {
	res, err := executor.ExecuteSyncInDir(dir, "git", "checkout", branch)
	if err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git checkout %s failed (exit %d): %s", branch, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}
