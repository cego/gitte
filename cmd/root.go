package cmd

import (
	"os"

	"github.com/spf13/cobra"
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
`}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gitte.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
