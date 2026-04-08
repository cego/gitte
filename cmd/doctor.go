package cmd

import (
	"fmt"

	"github.com/cego/gitte/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic checks",
		Long: `Run diagnostic checks and display a report.

Built-in checks:
  - git config    user.name and user.email are set
  - ssh           SSH connectivity to each configured source host
  - token         API tokens are configured for each source
  - directories   all configured project directories exist
  - startup       each configured startup check passes

Additional checks can be added in .gitte.yml under the 'doctor' key:

  doctor:
    docker-version:
      cmd: [docker, --version]
    node-version:
      type: shell
      script: node --version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			results := doctor.Run(globalCtx, globalCfg, globalCwd)
			pass := doctor.Print(results)
			if !pass {
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}
}
