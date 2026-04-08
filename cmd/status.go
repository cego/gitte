package cmd

import (
	"fmt"

	"github.com/cego/gitte/gitops"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local repository status",
		Long: `Show the local status of all configured repositories without contacting any remote.

Reports:
  - dirty repos (uncommitted changes)
  - repos with unpushed commits (ahead of origin/<default-branch>)
  - detached HEAD
  - repos not on the default branch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			statuses, err := gitops.Scan(globalCtx, globalCfg, globalCwd)
			if err != nil {
				return err
			}

			anyIssue := false
			for _, s := range statuses {
				var flags []string
				if s.Missing {
					flags = append(flags, "not cloned")
				} else {
					if s.Detached {
						flags = append(flags, "detached HEAD")
					} else if s.Branch != s.DefaultBranch {
						flags = append(flags, fmt.Sprintf("on branch %s", s.Branch))
					}
					if s.Dirty {
						flags = append(flags, "dirty")
					}
					if s.AheadCount > 0 {
						flags = append(flags, fmt.Sprintf("%d ahead of origin/%s", s.AheadCount, s.DefaultBranch))
					}
				}
				if len(flags) > 0 {
					anyIssue = true
					fmt.Printf("%-50s  %s\n", s.Name, joinFlags(flags))
				}
			}

			if !anyIssue {
				fmt.Printf("✓ All %d repositories clean\n", len(statuses))
			}
			return nil
		},
	}
}

func joinFlags(flags []string) string {
	result := ""
	for i, f := range flags {
		if i > 0 {
			result += ", "
		}
		result += f
	}
	return result
}
