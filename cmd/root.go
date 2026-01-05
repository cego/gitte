package cmd

import (
	"fmt"
	"gitte/config"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gitte",
	Short: "Tool for managing monorepo-like structured git repositories.",
	Long: `Gitte is a tool for managing monorepo-like structured git repositories with inter-project dependencies.

For example in a microservice environment
- Service A requires a database
- Service B requires service A and kafka
- Gitte will make sure that kafka and database starts first, then service a, then service b. Ensuring maximum parallelity in the dependency resolvement.

Gitte also contain other sub commands for utilities managing such a setup.
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		if cmd.Annotations == nil {
			return nil
		}

		if _, ok := cmd.Annotations["need-config"]; !ok {
			return nil
		}

		ctx := cmd.Context()

		cwd := viper.GetString("cwd")
		if cwd == "" {
			cwd = "."
		}

		fd, err := config.ResolveGitteDir(cwd)
		if err != nil {
			return fmt.Errorf("error resolving gitte dir: %w", err)
		}

		ctx = config.ContextWithCwd(ctx, fd.Directory)
		ctx, err = config.LoadCacheToContext(ctx, fd.Directory)
		if err != nil {
			return fmt.Errorf("error loading gitte cache: %w", err)
		}

		gitteConfig, err := config.LoadConfig(ctx, fd)
		if err != nil {
			return fmt.Errorf("error loading gitte config: %w", err)
		}

		cmd.SetContext(config.ContextWithConfig(ctx, gitteConfig))

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	viper.SetEnvPrefix("gitte")
	viper.AutomaticEnv()
	rootCmd.SilenceUsage = true
}
