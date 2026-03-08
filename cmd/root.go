package cmd

import (
	"context"
	"errors"
	"fmt"
	"gitte/config"
	"gitte/output"
	"gitte/state"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	flagConfigPath string
	flagNoTTY      bool
	flagCwd        string

	// Global shared state set during PersistentPreRunE
	globalCfg *config.GitteConfig
	globalSt  *state.GitteState
	globalCwd string
	globalCtx context.Context
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

// Execute adds all child commands and runs
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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
	)
}

func initGlobals() error {
	// Determine cwd: flag > GITTE_CWD env > process cwd
	cwd := flagCwd
	if cwd == "" {
		cwd = os.Getenv("GITTE_CWD")
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine working directory: %w", err)
		}
	}
	globalCwd = cwd
	globalCtx = context.Background()

	// Load state
	st, err := state.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	globalSt = st

	// Load config
	cfg, err := loadConfig(cwd)
	if err != nil {
		if errors.Is(err, config.ErrGitteConfigNotFound) {
			return fmt.Errorf("no .gitte.yml found (searched from %s)", cwd)
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve templates
	if err := config.ResolveTemplates(cfg); err != nil {
		return fmt.Errorf("template resolution failed: %w", err)
	}

	// Apply toggles
	cfg.FilterToggles(st.Toggles)

	globalCfg = cfg
	return nil
}

func loadConfig(cwd string) (*config.GitteConfig, error) {
	if flagConfigPath != "" {
		data, err := os.ReadFile(flagConfigPath)
		if err != nil {
			return nil, err
		}
		return config.LoadGitteConfigFromYAML(data)
	}

	fd, err := config.ResolveGitteDir(cwd)
	if err != nil {
		return nil, err
	}

	if fd.IsEnv {
		// Load remote config
		var cacheEntry *state.RemoteConfigCacheEntry
		if globalSt.Cache.RemoteConfig != nil {
			cacheEntry = globalSt.Cache.RemoteConfig
		}

		// Convert state cache to config.RemoteConfigCache
		var remoteCache *config.RemoteConfigCache
		if cacheEntry != nil {
			remoteCache = &config.RemoteConfigCache{
				RemoteGitRepo: cacheEntry.RemoteGitRepo,
				RemoteGitFile: cacheEntry.RemoteGitFile,
				RemoteGitRef:  cacheEntry.RemoteGitRef,
				FetchedAt:     cacheEntry.FetchedAt,
				Content:       cacheEntry.Content,
			}
		}

		cfg, newCache, err := config.LoadRemoteConfig(globalCtx, fd.ConfigContent, remoteCache)
		if err != nil {
			return nil, err
		}

		// Update state cache
		if newCache != nil {
			globalSt.Cache.RemoteConfig = &state.RemoteConfigCacheEntry{
				RemoteGitRepo: newCache.RemoteGitRepo,
				RemoteGitFile: newCache.RemoteGitFile,
				RemoteGitRef:  newCache.RemoteGitRef,
				FetchedAt:     newCache.FetchedAt,
				Content:       newCache.Content,
			}
			_ = state.Save(cwd, globalSt)
		}

		return cfg, nil
	}

	return config.LoadAndMergeConfig(fd)
}

// outputMode returns the current output mode
func outputMode() output.OutputMode {
	return output.DetectMode(flagNoTTY)
}

// withNeeds returns false when GITTE_NO_NEEDS=true, true otherwise
func withNeeds() bool {
	return os.Getenv("GITTE_NO_NEEDS") != "true"
}

// maxParallelization returns GITTE_MAX_TASK_PARALLELIZATION if set, else 0 (unlimited)
func maxParallelization() int {
	v := os.Getenv("GITTE_MAX_TASK_PARALLELIZATION")
	if v == "" {
		return 0
	}
	n := 0
	fmt.Sscanf(v, "%d", &n)
	return n
}
