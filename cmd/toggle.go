package cmd

import (
	"gitte/config"
	"gitte/internal"

	"github.com/spf13/cobra"
)

// toggleCmd represents the toggle command
var toggleCmd = &cobra.Command{
	Use: "toggle",
	Annotations: map[string]string{
		"need-config": "true",
	},
	Short: "Toggle projects on/off with an interactive TUI. Navigate with arrow keys and toggle with space.",
	Long: `Opens an interactive terminal UI where you can:
  - View all projects and their current state (enabled/disabled)
  - Toggle projects on/off using space or enter
  - Reset individual projects to their default state with 'r'
  - Reset all projects to default with 'R'
  - Navigate using arrow keys or vim-style j/k
  
Changes are saved automatically when you exit (q, esc, or Ctrl+C).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		cfg := config.ConfigFromContext(ctx)
		cwd := config.CwdFromContext(ctx)

		return internal.RunToggleTUI(cfg, cwd)
	},
}

func init() {
	rootCmd.AddCommand(toggleCmd)
}
