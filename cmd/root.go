package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cego/gitte/config"
	"github.com/cego/gitte/output"
	"github.com/cego/gitte/state"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	flagConfigPath string
	flagNoTTY      bool
	flagCwd        string
	flagNoNeeds    bool

	// Global shared state set during PersistentPreRunE
	globalCfg    *config.GitteConfig // toggle-filtered view
	globalRawCfg *config.GitteConfig // all projects, pre-filter (used by toggle TUI)
	globalSt     *state.GitteState
	globalCwd    string
	globalCtx    context.Context
	globalCancel context.CancelFunc
)

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:   "gitte",
	Short: "Developer environment orchestration tool",
	Long: `Gitte manages 50+ microservices across multiple git remotes,
running startup checks, syncing repos, and executing ordered actions
with dependency resolution.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initGlobals()
	},
}

var errorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

// SetVersion sets the version string shown by --version.
func SetVersion(v string) {
	rootCmd.Version = v
}

// Execute adds all child commands and runs
func Execute() {
	defer func() {
		if globalCancel != nil {
			globalCancel()
		}
	}()
	err := rootCmd.Execute()
	if err != nil {
		if output.DetectMode(flagNoTTY) == output.ModePlain {
			fmt.Fprintln(os.Stderr, "error:", err)
		} else {
			fmt.Fprintln(os.Stderr, errorStyle.Render("error:")+" "+err.Error())
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfigPath, "config", "", "path to .gitte.yml (default: auto-discover)")
	rootCmd.PersistentFlags().BoolVar(&flagNoTTY, "no-tty", false, "disable TUI (plain output)")
	rootCmd.PersistentFlags().StringVar(&flagCwd, "cwd", "", "working directory (default: current dir)")

	rootCmd.AddCommand(
		newValidateCmd(),
		newStartupCmd(),
		newGitopsCmd(),
		newActionsCmd(),
		newRunCmd(),
		newToggleCmd(),
		newFeaturesCmd(),
		newCleanCmd(),
		newListCmd(),
		newSourcesCmd(),
	)

	// --config accepts a file path
	_ = rootCmd.RegisterFlagCompletionFunc("config", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"yml", "yaml"}, cobra.ShellCompDirectiveFilterFileExt
	})
}

func initGlobals() error {
	// Determine starting search dir: flag > GITTE_CWD env > process cwd
	searchDir := flagCwd
	if searchDir == "" {
		searchDir = os.Getenv("GITTE_CWD")
	}
	if searchDir == "" {
		var err error
		searchDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine working directory: %w", err)
		}
	}
	globalCtx, globalCancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// loadConfig resolves the config directory, sets globalCwd, loads state, then loads config
	cfg, err := loadConfig(searchDir)
	if err != nil {
		if errors.Is(err, config.ErrGitteConfigNotFound) {
			return fmt.Errorf("no .gitte.yml found (searched from %s)", searchDir)
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve templates
	if err := config.ResolveTemplates(cfg); err != nil {
		return fmt.Errorf("template resolution failed: %w", err)
	}

	// Keep the full config before toggle filtering (needed by the toggle TUI).
	globalRawCfg = cfg
	globalCfg = cfg.WithTogglesApplied(globalSt.Toggles)
	return nil
}

func loadConfig(searchDir string) (*config.GitteConfig, error) {
	if flagConfigPath != "" {
		data, err := os.ReadFile(flagConfigPath)
		if err != nil {
			return nil, err
		}
		globalCwd = filepath.Dir(flagConfigPath)
		if err := loadState(); err != nil {
			return nil, err
		}
		return config.LoadGitteConfigFromYAML(data)
	}

	fd, err := config.ResolveGitteDir(searchDir)
	if err != nil {
		return nil, err
	}

	// globalCwd is the directory containing the config — state lives here too
	globalCwd = fd.Directory
	if err := loadState(); err != nil {
		return nil, err
	}

	if fd.IsEnv {
		var remoteCache *config.RemoteConfigCache
		if globalSt.Cache.RemoteConfig != nil {
			c := globalSt.Cache.RemoteConfig
			remoteCache = &config.RemoteConfigCache{
				RemoteGitRepo: c.RemoteGitRepo,
				RemoteGitFile: c.RemoteGitFile,
				RemoteGitRef:  c.RemoteGitRef,
				FetchedAt:     c.FetchedAt,
				Content:       c.Content,
			}
		}

		verbose := outputMode() == output.ModePlain
		saveCache := func(newCache *config.RemoteConfigCache) {
			globalSt.Cache.RemoteConfig = &state.RemoteConfigCacheEntry{
				RemoteGitRepo: newCache.RemoteGitRepo,
				RemoteGitFile: newCache.RemoteGitFile,
				RemoteGitRef:  newCache.RemoteGitRef,
				FetchedAt:     newCache.FetchedAt,
				Content:       newCache.Content,
			}
			stateFile := filepath.Join(globalCwd, state.StateFileName)
			if err := state.Save(globalCwd, globalSt); err != nil {
				fmt.Fprintf(os.Stderr, "[remote config] failed to save cache to %s: %v\n", stateFile, err)
			} else if verbose {
				fmt.Fprintf(os.Stderr, "[remote config] saved cache to %s (fetched_at=%s)\n", stateFile, newCache.FetchedAt.Format(time.RFC3339))
			}
		}

		cfg, newCache, err := config.LoadRemoteConfig(globalCtx, fd.ConfigContent, remoteCache, saveCache, verbose)
		if err != nil {
			return nil, err
		}
		if newCache != nil {
			saveCache(newCache)
		}

		return cfg, nil
	}

	return config.LoadAndMergeConfig(fd)
}

func loadState() error {
	st, err := state.Load(globalCwd)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	globalSt = st
	return nil
}

// outputMode returns the current output mode
func outputMode() output.OutputMode {
	return output.DetectMode(flagNoTTY)
}

// withNeeds returns false when --no-needs flag is set or GITTE_NO_NEEDS=true, true otherwise
func withNeeds() bool {
	return !flagNoNeeds && os.Getenv("GITTE_NO_NEEDS") != "true"
}

// maxParallelization returns GITTE_MAX_TASK_PARALLELIZATION if set, else 0 (unlimited)
func maxParallelization() int {
	v := os.Getenv("GITTE_MAX_TASK_PARALLELIZATION")
	if v == "" {
		return 0
	}
	n := 0
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0
	}
	return n
}
