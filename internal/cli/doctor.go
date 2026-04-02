package cli

import (
	"fmt"

	"github.com/dshills/atlas/internal/doctor"
	"github.com/dshills/atlas/internal/output"
	"github.com/spf13/cobra"
)

// DoctorCmd creates the `atlas doctor` command.
func DoctorCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check repository health",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			report := doctor.Run(repoRoot)

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, c := range report.Checks {
					val := c.Status
					if c.Details != "" {
						val = fmt.Sprintf("%s — %s", c.Status, c.Details)
					}
					kvs = append(kvs, output.KV{Key: c.Name, Value: val})
				}
				status := "HEALTHY"
				if !report.Healthy {
					status = "UNHEALTHY"
				}
				kvs = append(kvs, output.KV{Key: "Overall", Value: status})
				return f.WriteText(kvs)
			}
			return f.Write(report)
		},
	}
}
