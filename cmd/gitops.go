package cmd

import (
	"fmt"
	"gitte/config"
	"gitte/internal"
	"os"

	"github.com/spf13/cobra"
)

// gitopsCmd represents the gitops command
var gitopsCmd = &cobra.Command{
	Use:   "gitops",
	Short: "GitOps refer to GitOperations and will ensure that the enabled projects are cloned and up to date if possible.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("gitops called")
		fs := os.DirFS(".")
		fd, err := config.ResolveGitteDir(fs)
		if err != nil {
			return fmt.Errorf("error resolving gitte dir: %w", err)
		}

		gitteConfig, err := config.LoadConfig(fd)
		if err != nil {
			return fmt.Errorf("error loading gitte config: %w", err)
		}

		//fmt.Println("Loaded Gitte Config:", gitteConfig)

		cwd := "/home/lejo/gitte-test" // TODO move config loading and cwd detection to shared place

		return internal.GitOps(cmd.Context(), cwd, *gitteConfig)
	},
}

func init() {
	rootCmd.AddCommand(gitopsCmd)
}
