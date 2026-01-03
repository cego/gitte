package cmd

import (
	"fmt"
	"gitte/config"
	"os"

	"github.com/spf13/cobra"
)

// gitopsCmd represents the gitops command
var gitopsCmd = &cobra.Command{
	Use:   "gitops",
	Short: "GitOps refer to GitOperations and will ensure that the enabled projects are cloned and up to date if possible.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("gitops called")
		fs := os.DirFS(".")
		fd, err := config.ResolveGitteDir(fs)
		if err != nil {
			fmt.Println("Error resolving .gitte dir:", err)
			return
		}

		gitteConfig, err := config.LoadConfig(fd)
		if err != nil {
			fmt.Println("Error loading gitte config:", err)
			return
		}

		fmt.Println("Loaded Gitte Config:", gitteConfig)
	},
}

func init() {
	rootCmd.AddCommand(gitopsCmd)
}
