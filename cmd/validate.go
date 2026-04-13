package cmd

import (
	"fmt"
	"os"

	"github.com/cego/gitte/config"

	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long:  "Parse config, validate schema, detect cycles, and report missing references.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate()
		},
	}
}

func runValidate() error {
	result := config.ValidateConfig(globalCfg)

	for _, warn := range result.Warnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warn.Error())
	}

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", e.Error())
	}

	if result.HasErrors() {
		return fmt.Errorf("config has %d error(s)", len(result.Errors))
	}

	fmt.Printf("Config is valid (%d project(s), %d warning(s))\n",
		len(globalCfg.Projects), len(result.Warnings))
	return nil
}
