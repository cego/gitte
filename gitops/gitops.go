package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/executor"
	"github.com/cego/gitte/output"
)

// parallelLimit returns the effective parallelization cap for gitops clone/pull
// tasks. It uses GITTE_MAX_TASK_PARALLELIZATION if set; otherwise it falls
// back to defaultVal (0 = unlimited).
func parallelLimit(defaultVal int) int {
	v := os.Getenv("GITTE_MAX_TASK_PARALLELIZATION")
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}

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
	warnFn func(string),
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	projectNames := make([]string, 0, len(cfg.Projects))
	for name := range cfg.Projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	taskNames := make([]string, len(projectNames))
	dirs := make(map[string]string, len(projectNames))
	for i, n := range projectNames {
		taskNames[i] = "gitops:" + n
		proj := cfg.Projects[n]
		if localDir, err := config.LocalDirForRemote(proj.Remote); err == nil {
			dirs[taskNames[i]] = localDir
		}
	}

	view := newView(mode, "Syncing repositories", taskNames, dirs, cancel)

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
				return syncProject(ctx, cwd, name, proj, noRebase, setDetail, addPrompt, warnFn)
			},
		})
	}

	exec, err := executor.NewExecutor(tasks, executor.ExecutorOptions{
		MaxParallelization: parallelLimit(0),
		OnTaskStart:        view.OnStart,
		OnTaskFinish:       view.OnFinish,
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
				if err := checkoutBranch(ctx, p.ProjectPath, p.DefaultBranch); err != nil {
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
		return cloneRemote(ctx, cwd, remote, localDir)
	}

	if err := checkRepositorySafety(ctx, projectPath); err != nil {
		return err
	}
	if err := fetchOrigin(ctx, projectPath); err != nil {
		return fmt.Errorf("fetching %s: %w", localDir, err)
	}

	branch := getCurrentBranch(ctx, projectPath)
	if branch == "" {
		return fmt.Errorf("cannot determine current branch in %s", projectPath)
	}

	_, err = mergeFastForward(ctx, projectPath, "origin/"+branch)
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
	warnFn func(string),
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
	// The TUI is already gone by the time this runs, so we print to stderr.
	retryFn := func() error {
		return syncProject(ctx, cwd, name, proj, noRebase,
			func(d string) { fmt.Fprintf(os.Stderr, "  [%s] %s\n", name, d) },
			func(_ CheckoutPrompt) {}, // no further prompts after retry
			func(w string) { fmt.Fprintln(os.Stderr, "warning: "+w) },
		)
	}

	// Clone if directory does not exist yet.
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		setDetail("cloning…")
		if err := cloneRemote(ctx, cwd, proj.Remote, localDir); err != nil {
			return err
		}
		setDetail("cloned")
		return nil
	}

	if err := checkRepositorySafety(ctx, projectPath); err != nil {
		return err
	}

	// Always fetch so remote refs are fresh.
	if err := fetchOrigin(ctx, projectPath); err != nil {
		warnFn(fmt.Sprintf("[%s] fetch failed: %v", name, err))
		return fmt.Errorf("[%s] fetch failed: %w", name, err)
	}

	// Detached HEAD.
	detached, err := isDetachedHEAD(ctx, projectPath)
	if err != nil {
		return err
	}
	if detached {
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

	currentBranch := getCurrentBranch(ctx, projectPath)

	// ── On default branch ──────────────────────────────────────────────────
	if currentBranch == defaultBranch {
		upToDate, err := mergeFastForward(ctx, projectPath, "origin/"+defaultBranch)
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
	if !remoteRefExists(ctx, projectPath, remoteCurrentRef) && hasRemoteTrackingConfig(ctx, projectPath, currentBranch) {
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

	// Pull from the remote tracking branch and rebase in one transaction so a
	// failure restores the state from before either operation.
	pulledLabel := ""
	var rebaseConflict bool
	transactionErr := withTrackedChanges(ctx, projectPath, func() error {
		if remoteRefExists(ctx, projectPath, remoteCurrentRef) {
			ahead := commitsAhead(ctx, projectPath, "HEAD", remoteCurrentRef)
			if ahead > 0 {
				if _, err := mergeFastForwardRaw(ctx, projectPath, remoteCurrentRef); err != nil {
					var diverged *fastForwardDivergedError
					if !errors.As(err, &diverged) {
						return err
					}
					setDetail(fmt.Sprintf("stale: diverged from origin/%s", currentBranch))
					return err
				} else {
					pulledLabel = fmt.Sprintf("pulled %d from origin/%s", ahead, currentBranch)
				}
			}
		}

		// Auto-rebase onto default branch (unless disabled).
		if !noRebase {
			remoteDefaultRef := "origin/" + defaultBranch
			if commitsAhead(ctx, projectPath, "HEAD", remoteDefaultRef) > 0 {
				if err := prepareRebase(ctx, projectPath, remoteDefaultRef); err != nil {
					return err
				}
				if _, err := rebaseRaw(ctx, projectPath, remoteDefaultRef); err != nil {
					var conflict *rebaseConflictError
					if errors.As(err, &conflict) {
						rebaseConflict = true
					}
					return err
				}
			}
		}
		return nil
	})
	if transactionErr != nil {
		var diverged *fastForwardDivergedError
		if errors.As(transactionErr, &diverged) {
			return nil
		}
		var conflict *rebaseConflictError
		if !rebaseConflict || !errors.As(transactionErr, &conflict) {
			return transactionErr
		}
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
	if rebaseConflict {
		return fmt.Errorf("rebase conflict was not reported")
	}

	// Stale check for projects not yet brought up to date.
	if !addStaleIfNeeded(ctx, name, projectPath, defaultBranch, retryFn, setDetail, addPrompt) {
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
func addStaleIfNeeded(ctx context.Context, name, dir, defaultBranch string, retryFn func() error, setDetail func(string), addPrompt func(CheckoutPrompt)) bool {
	days := staleDays(ctx, dir, defaultBranch)
	if days == 0 {
		return false
	}

	hasWork := commitsAhead(ctx, dir, "origin/"+defaultBranch, "HEAD") > 0

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
func staleDays(ctx context.Context, dir, defaultBranch string) int {
	if defaultBranch == "" {
		defaultBranch = "master"
	}
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return 0
	}
	if b := strings.TrimSpace(string(res.Stdout)); b == "HEAD" || b == defaultBranch {
		return 0
	}

	remoteRef := "origin/" + defaultBranch
	res2, err := executor.ExecuteSyncInDir(ctx, dir, "git", "log", "HEAD.."+remoteRef, "--format=%ct", "--max-count=1")
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
	if err := checkRepositorySafety(ctx, dir); err != nil {
		return err
	}
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	res, err := runGit(fetchCtx, dir, true, "fetch", "--atomic", "origin")
	if err != nil {
		if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git fetch timed out after %s", fetchTimeout)
		}
		if fetchCtx.Err() != nil {
			return fetchCtx.Err()
		}
		return fmt.Errorf("git fetch: %w", err)
	}
	if res.ExitCode != 0 {
		if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git fetch timed out after %s", fetchTimeout)
		}
		if fetchCtx.Err() != nil {
			return fetchCtx.Err()
		}
		return fmt.Errorf("git fetch failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

func getCurrentBranch(ctx context.Context, dir string) string {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(string(res.Stdout))
}

func remoteRefExists(ctx context.Context, dir, ref string) bool {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "rev-parse", "--verify", ref)
	return err == nil && res.ExitCode == 0
}

func hasRemoteTrackingConfig(ctx context.Context, dir, branch string) bool {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "config", "--get", "branch."+branch+".remote")
	return err == nil && res.ExitCode == 0 && len(strings.TrimSpace(string(res.Stdout))) > 0
}

// commitsAhead returns the number of commits reachable from target but not from base.
func commitsAhead(ctx context.Context, dir, base, target string) int {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "rev-list", "--count", base+".."+target)
	if err != nil || res.ExitCode != 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(res.Stdout)))
	if err != nil {
		return 0
	}
	return n
}

func isDetachedHEAD(ctx context.Context, dir string) (bool, error) {
	res, err := executor.ExecuteSyncInDir(ctx, dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return false, fmt.Errorf("git rev-parse: %w", err)
	}
	if res.ExitCode != 0 {
		return false, nil
	}
	return strings.TrimSpace(string(res.Stdout)) == "HEAD", nil
}

func hasTrackedChanges(ctx context.Context, dir string) (bool, error) {
	res, err := runGit(ctx, dir, false, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, fmt.Errorf("git status failed (exit %d)", res.ExitCode)
	}
	return len(strings.TrimSpace(string(res.Stdout))) > 0, nil
}

func cloneRemote(ctx context.Context, cwd, remote, localDir string) error {
	res, err := runGit(ctx, cwd, true, "clone", remote, localDir)
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
func mergeFastForward(ctx context.Context, dir, ref string) (upToDate bool, err error) {
	var result bool
	err = withTrackedChanges(ctx, dir, func() error {
		var rawErr error
		result, rawErr = mergeFastForwardRaw(ctx, dir, ref)
		return rawErr
	})
	return result, err
}

type fastForwardDivergedError struct{}

func (e *fastForwardDivergedError) Error() string {
	return "cannot fast-forward; branches have diverged"
}

func mergeFastForwardRaw(ctx context.Context, dir, ref string) (upToDate bool, err error) {
	res, err := runGit(ctx, dir, true, "merge", "--ff-only", "--no-overwrite-ignore", ref)
	if err != nil {
		return false, fmt.Errorf("git merge: %w", err)
	}
	if res.ExitCode != 0 {
		stderr := string(res.Stderr)
		if regexp.MustCompile(`(?i)(not possible to fast.forward|cannot fast.forward|needs merge)`).MatchString(stderr) {
			return false, &fastForwardDivergedError{}
		}
		return false, fmt.Errorf("git merge --ff-only failed (exit %d): %s", res.ExitCode, strings.TrimSpace(stderr))
	}
	return strings.Contains(string(res.Stdout), "Already up to date."), nil
}

// tryRebase attempts git rebase <onto>, preserving tracked changes transactionally.
func tryRebase(ctx context.Context, dir, onto string) (bool, error) {
	if err := prepareRebase(ctx, dir, onto); err != nil {
		return false, err
	}
	var result bool
	err := withTrackedChanges(ctx, dir, func() error {
		var rawErr error
		result, rawErr = rebaseRaw(ctx, dir, onto)
		return rawErr
	})
	var conflict *rebaseConflictError
	if errors.As(err, &conflict) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return result, err
}

func checkoutBranch(ctx context.Context, dir, branch string) error {
	if err := checkRepositorySafety(ctx, dir); err != nil {
		return err
	}
	originalHead, err := gitOutput(ctx, dir, false, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("recording HEAD before checkout: %w", err)
	}
	originalBranch := getCurrentBranch(ctx, dir)
	dirty, err := hasTrackedChanges(ctx, dir)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("cannot checkout %s with tracked changes", branch)
	}
	res, err := runGit(ctx, dir, true, "checkout", "--no-overwrite-ignore", branch)
	if err != nil {
		if ctx.Err() != nil {
			if restoreErr := restoreCheckout(dir, originalBranch, strings.TrimSpace(originalHead)); restoreErr != nil {
				return fmt.Errorf("checkout canceled: %v; restoring checkout: %w", ctx.Err(), restoreErr)
			}
			return ctx.Err()
		}
		return fmt.Errorf("git checkout: %w", err)
	}
	if res.ExitCode != 0 {
		if ctx.Err() != nil {
			if restoreErr := restoreCheckout(dir, originalBranch, strings.TrimSpace(originalHead)); restoreErr != nil {
				return fmt.Errorf("checkout canceled: %v; restoring checkout: %w", ctx.Err(), restoreErr)
			}
			return ctx.Err()
		}
		return fmt.Errorf("git checkout %s failed (exit %d): %s", branch, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	if ctx.Err() != nil {
		if restoreErr := restoreCheckout(dir, originalBranch, strings.TrimSpace(originalHead)); restoreErr != nil {
			return fmt.Errorf("checkout canceled: %v; restoring checkout: %w", ctx.Err(), restoreErr)
		}
		return ctx.Err()
	}
	return nil
}

func restoreCheckout(dir, branch, head string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	args := []string{"checkout", "--no-overwrite-ignore"}
	if branch == "" || branch == "HEAD" {
		args = append(args, "--detach", head)
	} else {
		args = append(args, branch)
	}
	res, err := runGit(ctx, dir, true, args...)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git checkout failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	actual, err := gitOutput(ctx, dir, false, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(actual) != head {
		return fmt.Errorf("HEAD was not restored")
	}
	return nil
}

const cleanupTimeout = 10 * time.Second

type rebaseConflictError struct {
	message string
}

func (e *rebaseConflictError) Error() string { return e.message }

type rebaseBlockedError struct {
	reason string
}

func (e *rebaseBlockedError) Error() string {
	return "automatic rebase blocked: " + e.reason
}

type repoSnapshot struct {
	head          string
	status        string
	trackedStatus string
	stagedDiff    string
	unstagedDiff  string
}

type trackedStash struct {
	commit string
}

func runGit(ctx context.Context, dir string, mutating bool, args ...string) (*executor.ExecuteResult, error) {
	if mutating {
		args = append([]string{"-c", "core.hooksPath=" + os.DevNull}, args...)
	}
	return executor.ExecuteSyncInDir(ctx, dir, "git", args...)
}

func withTrackedChanges(ctx context.Context, dir string, operation func() error) error {
	if err := checkRepositorySafety(ctx, dir); err != nil {
		return err
	}

	snapshot, err := captureRepoSnapshot(ctx, dir)
	if err != nil {
		return err
	}
	stash, err := stashTrackedChanges(dir, snapshot.trackedStatus != "")
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
			return fmt.Errorf("operation canceled: %v; repository restoration failed: %w", err, restoreErr)
		}
		return err
	}

	opErr := operation()
	if err := ctx.Err(); err != nil {
		if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
			return fmt.Errorf("operation canceled: %v; repository restoration failed: %w", err, restoreErr)
		}
		return err
	}
	if opErr != nil {
		if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
			return fmt.Errorf("operation failed: %v; repository restoration failed: %w", opErr, restoreErr)
		}
		return opErr
	}
	if operation, err := activeGitOperation(context.Background(), dir); err != nil {
		if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
			return fmt.Errorf("checking git operation state failed: %v; repository restoration failed: %w", err, restoreErr)
		}
		return err
	} else if operation != "" {
		if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
			return fmt.Errorf("git operation %s remained active; repository restoration failed: %w", operation, restoreErr)
		}
		return fmt.Errorf("git operation %s remained active", operation)
	}

	if stash != nil {
		if err := applyTrackedStash(context.Background(), dir, stash); err != nil {
			if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
				return fmt.Errorf("tracked changes could not be reapplied: %v; repository restoration failed: %w", err, restoreErr)
			}
			return fmt.Errorf("tracked changes could not be reapplied: %w", err)
		}
		if err := ctx.Err(); err != nil {
			if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
				return fmt.Errorf("operation canceled: %v; repository restoration failed: %w", err, restoreErr)
			}
			return err
		}
		if err := verifyTrackedState(context.Background(), dir, snapshot.trackedStatus); err != nil {
			if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
				return fmt.Errorf("tracked state verification failed: %v; repository restoration failed: %w", err, restoreErr)
			}
			return fmt.Errorf("tracked state verification failed: %w", err)
		}
		if err := ctx.Err(); err != nil {
			if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
				return fmt.Errorf("operation canceled: %v; repository restoration failed: %w", err, restoreErr)
			}
			return err
		}
		if err := removeTrackedStash(context.Background(), dir, stash); err != nil {
			if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
				return fmt.Errorf("temporary stash cleanup failed: %v; repository restoration failed: %w", err, restoreErr)
			}
			return fmt.Errorf("temporary stash cleanup failed: %w", err)
		}
	} else if err := ctx.Err(); err != nil {
		if restoreErr := restoreAndRemoveStash(dir, snapshot, stash); restoreErr != nil {
			return fmt.Errorf("operation canceled: %v; repository restoration failed: %w", err, restoreErr)
		}
		return err
	}
	return nil
}

func restoreAndRemoveStash(dir string, snapshot repoSnapshot, stash *trackedStash) error {
	if err := restoreRepository(dir, snapshot, stash); err != nil {
		return err
	}
	if stash != nil {
		if err := removeTrackedStash(context.Background(), dir, stash); err != nil {
			return err
		}
	}
	return nil
}

func captureRepoSnapshot(ctx context.Context, dir string) (repoSnapshot, error) {
	head, err := gitOutput(ctx, dir, false, "rev-parse", "HEAD")
	if err != nil {
		return repoSnapshot{}, fmt.Errorf("recording HEAD: %w", err)
	}
	status, err := gitOutput(ctx, dir, false, "status", "--porcelain=v2", "--untracked-files=normal")
	if err != nil {
		return repoSnapshot{}, fmt.Errorf("recording repository status: %w", err)
	}
	trackedStatus, err := gitOutput(ctx, dir, false, "status", "--porcelain=v1", "--untracked-files=no")
	if err != nil {
		return repoSnapshot{}, fmt.Errorf("recording tracked status: %w", err)
	}
	unstaged, err := gitOutput(ctx, dir, false, "diff", "--binary")
	if err != nil {
		return repoSnapshot{}, fmt.Errorf("recording unstaged changes: %w", err)
	}
	staged, err := gitOutput(ctx, dir, false, "diff", "--cached", "--binary")
	if err != nil {
		return repoSnapshot{}, fmt.Errorf("recording staged changes: %w", err)
	}
	return repoSnapshot{
		head:          strings.TrimSpace(head),
		status:        status,
		trackedStatus: trackedStatus,
		stagedDiff:    staged,
		unstagedDiff:  unstaged,
	}, nil
}

func stashTrackedChanges(dir string, dirty bool) (*trackedStash, error) {
	if !dirty {
		return nil, nil
	}
	// Once stash creation starts, let it finish even if the parent is canceled;
	// the caller restores it at the next safe checkpoint.
	res, err := runGit(context.Background(), dir, true, "stash", "push", "--message", "gitte automatic sync")
	if err != nil {
		return nil, fmt.Errorf("stashing tracked changes: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("stashing tracked changes failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	commit, err := gitOutput(context.Background(), dir, false, "rev-parse", "--verify", "refs/stash")
	if err != nil {
		return nil, fmt.Errorf("recording temporary stash: %w", err)
	}
	return &trackedStash{commit: strings.TrimSpace(commit)}, nil
}

func applyTrackedStash(ctx context.Context, dir string, stash *trackedStash) error {
	res, err := runGit(ctx, dir, true, "stash", "apply", "--index", stash.commit)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git stash apply failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

func removeTrackedStash(ctx context.Context, dir string, stash *trackedStash) error {
	current, err := gitOutput(ctx, dir, false, "rev-parse", "--verify", "refs/stash")
	if err != nil || strings.TrimSpace(current) != stash.commit {
		return fmt.Errorf("temporary stash is no longer the top stash")
	}
	res, err := runGit(ctx, dir, true, "stash", "drop", "refs/stash@{0}")
	if err == nil && res.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("git stash drop failed; temporary stash retained for recovery")
}

func verifyTrackedState(ctx context.Context, dir, expected string) error {
	actual, err := gitOutput(ctx, dir, false, "status", "--porcelain=v1", "--untracked-files=no")
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("staged or unstaged state changed")
	}
	return nil
}

func restoreRepository(dir string, snapshot repoSnapshot, stash *trackedStash) error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	if operation, err := activeGitOperation(ctx, dir); err != nil {
		return err
	} else if operation == "rebase" {
		res, runErr := runGit(ctx, dir, true, "rebase", "--abort")
		if runErr != nil {
			return fmt.Errorf("aborting rebase: %w", runErr)
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("aborting rebase failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
		}
	} else if operation != "" {
		return fmt.Errorf("unexpected git operation %s is active", operation)
	}
	if operation, err := activeGitOperation(ctx, dir); err != nil {
		return err
	} else if operation != "" {
		return fmt.Errorf("git %s operation remains active after cleanup", operation)
	}

	res, err := runGit(ctx, dir, true, "reset", "--hard", snapshot.head)
	if err != nil {
		return fmt.Errorf("resetting HEAD: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("resetting HEAD failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	if stash != nil {
		if err := applyTrackedStash(ctx, dir, stash); err != nil {
			return fmt.Errorf("restoring tracked changes: %w", err)
		}
	}
	if err := verifySnapshot(ctx, dir, snapshot); err != nil {
		return err
	}
	return nil
}

func verifySnapshot(ctx context.Context, dir string, expected repoSnapshot) error {
	actual, err := captureRepoSnapshot(ctx, dir)
	if err != nil {
		return err
	}
	if actual.head != expected.head || actual.status != expected.status ||
		actual.stagedDiff != expected.stagedDiff || actual.unstagedDiff != expected.unstagedDiff {
		return fmt.Errorf("repository state does not match its original snapshot")
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, mutating bool, args ...string) (string, error) {
	res, err := runGit(ctx, dir, mutating, args...)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("git %s failed (exit %d): %s", args[0], res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return string(res.Stdout), nil
}

func rebaseRaw(ctx context.Context, dir, onto string) (bool, error) {
	res, err := runGit(ctx, dir, true, "rebase", "--no-autostash", onto)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, fmt.Errorf("git rebase: %w", err)
	}
	if res.ExitCode == 0 {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	rebaseOutput := strings.TrimSpace(string(res.Stderr))
	if rebaseOutput == "" {
		rebaseOutput = strings.TrimSpace(string(res.Stdout))
	}
	operation, stateErr := activeGitOperation(context.Background(), dir)
	if stateErr != nil {
		return false, fmt.Errorf("git rebase failed (exit %d): %s; checking rebase state: %w", res.ExitCode, rebaseOutput, stateErr)
	}
	if operation == "rebase" {
		return false, &rebaseConflictError{message: fmt.Sprintf("git rebase failed (exit %d): %s", res.ExitCode, rebaseOutput)}
	}
	return false, fmt.Errorf("git rebase failed (exit %d): %s", res.ExitCode, rebaseOutput)
}

func prepareRebase(ctx context.Context, dir, onto string) error {
	if err := rebasePreflight(ctx, dir, onto); err != nil {
		return err
	}
	head, err := gitOutput(ctx, dir, false, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("recording rebase backup: %w", err)
	}
	branch := getCurrentBranch(ctx, dir)
	if branch == "" || branch == "HEAD" {
		return &rebaseBlockedError{reason: "the current HEAD is not a local branch"}
	}
	backupRef := "refs/gitte/backups/" + branch + "/" + strings.TrimSpace(head)
	res, err := runGit(ctx, dir, true, "update-ref", backupRef, strings.TrimSpace(head))
	if err != nil {
		return fmt.Errorf("creating rebase backup ref: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("creating rebase backup ref failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

func rebasePreflight(ctx context.Context, dir, onto string) error {
	if err := checkRepositorySafety(ctx, dir); err != nil {
		return err
	}
	merges, err := gitOutput(ctx, dir, false, "rev-list", "--merges", "--count", onto+"..HEAD")
	if err != nil {
		return fmt.Errorf("checking for local merge commits: %w", err)
	}
	if strings.TrimSpace(merges) != "0" {
		return &rebaseBlockedError{reason: "the branch contains local merge commits"}
	}

	paths := make(map[string]struct{})
	addPaths := func(data string) {
		for _, path := range strings.Split(data, "\x00") {
			if path != "" {
				paths[path] = struct{}{}
			}
		}
	}
	changed, err := gitOutput(ctx, dir, false, "diff", "--name-only", "-z", "HEAD", onto)
	if err != nil {
		return fmt.Errorf("checking rebase checkout paths: %w", err)
	}
	addPaths(changed)
	commits, err := gitOutput(ctx, dir, false, "rev-list", "--reverse", onto+"..HEAD")
	if err != nil {
		return fmt.Errorf("listing commits for rebase: %w", err)
	}
	for _, commit := range strings.Fields(commits) {
		changed, err := gitOutput(ctx, dir, false, "diff-tree", "--root", "--no-commit-id", "--name-only", "-r", "-z", commit)
		if err != nil {
			return fmt.Errorf("checking rebase commit paths: %w", err)
		}
		addPaths(changed)
	}

	localPaths, err := localUntrackedPaths(ctx, dir)
	if err != nil {
		return err
	}
	ignoreCase, err := repositoryIgnoresCase(ctx, dir)
	if err != nil {
		return err
	}
	for local := range localPaths {
		for changed := range paths {
			if pathCollides(local, changed, ignoreCase) {
				return &rebaseBlockedError{reason: fmt.Sprintf("local untracked or ignored path %q collides with replay path %q", local, changed)}
			}
		}
	}
	return nil
}

func localUntrackedPaths(ctx context.Context, dir string) (map[string]struct{}, error) {
	paths := make(map[string]struct{})
	for _, args := range [][]string{
		{"ls-files", "--others", "--exclude-standard", "-z"},
		{"ls-files", "--others", "--ignored", "--exclude-standard", "-z"},
	} {
		output, err := gitOutput(ctx, dir, false, args...)
		if err != nil {
			return nil, fmt.Errorf("listing untracked and ignored paths: %w", err)
		}
		for _, path := range strings.Split(output, "\x00") {
			if path != "" {
				paths[path] = struct{}{}
			}
		}
	}
	return paths, nil
}

func pathCollides(left, right string, ignoreCase bool) bool {
	if ignoreCase {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func repositoryIgnoresCase(ctx context.Context, dir string) (bool, error) {
	res, err := runGit(ctx, dir, false, "config", "--bool", "core.ignorecase")
	if err != nil {
		return false, err
	}
	if res.ExitCode == 1 {
		return false, nil
	}
	if res.ExitCode != 0 {
		return false, fmt.Errorf("reading core.ignorecase failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	value, err := strconv.ParseBool(strings.TrimSpace(string(res.Stdout)))
	if err != nil {
		return false, fmt.Errorf("parsing core.ignorecase: %w", err)
	}
	return value, nil
}

func checkRepositorySafety(ctx context.Context, dir string) error {
	if operation, err := activeGitOperation(ctx, dir); err != nil {
		return err
	} else if operation != "" {
		return fmt.Errorf("git %s operation already in progress", operation)
	}

	res, err := runGit(ctx, dir, false, "status", "--porcelain=v1", "-z", "--ignore-submodules=none")
	if err != nil {
		return fmt.Errorf("checking repository status: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git status failed (exit %d): %s", res.ExitCode, strings.TrimSpace(string(res.Stderr)))
	}
	submodules, err := gitlinkPaths(ctx, dir)
	if err != nil {
		return err
	}
	for _, record := range strings.Split(string(res.Stdout), "\x00") {
		if len(record) < 4 {
			continue
		}
		status := record[:2]
		path := record[3:]
		for submodule := range submodules {
			if path == submodule && status != "  " {
				return fmt.Errorf("dirty submodule %s", submodule)
			}
		}
		if strings.ContainsAny(status, "Uu") {
			return fmt.Errorf("unmerged changes are present")
		}
	}
	return nil
}

func gitlinkPaths(ctx context.Context, dir string) (map[string]struct{}, error) {
	output, err := gitOutput(ctx, dir, false, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, fmt.Errorf("checking submodules: %w", err)
	}
	paths := make(map[string]struct{})
	for _, record := range strings.Split(output, "\x00") {
		parts := strings.SplitN(record, "\t", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "160000 ") {
			paths[parts[1]] = struct{}{}
		}
	}
	return paths, nil
}

func activeGitOperation(ctx context.Context, dir string) (string, error) {
	operations := []struct {
		name string
		path string
	}{
		{name: "rebase", path: "rebase-merge"},
		{name: "rebase", path: "rebase-apply"},
		{name: "merge", path: "MERGE_HEAD"},
		{name: "cherry-pick", path: "CHERRY_PICK_HEAD"},
		{name: "revert", path: "REVERT_HEAD"},
		{name: "sequencer", path: "sequencer"},
		{name: "bisect", path: "BISECT_START"},
	}
	for _, operation := range operations {
		path, err := gitOutput(ctx, dir, false, "rev-parse", "--git-path", operation.path)
		if err != nil {
			return "", fmt.Errorf("checking git operation state: %w", err)
		}
		path = strings.TrimSpace(path)
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return operation.name, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("checking %s: %w", operation.path, err)
		}
	}
	return "", nil
}
