package cli

import (
	"fmt"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/validate"
	"github.com/spf13/cobra"
)

// ValidateCmd creates the `atlas validate` command.
func ValidateCmd(ctx *CLIContext) *cobra.Command {
	var flagStrict bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate index integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}

			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			cfg, err := config.LoadFromDir(repoRoot)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			opts := validate.Options{
				Strict:   flagStrict,
				RepoRoot: repoRoot,
				Include:  cfg.Include,
				Exclude:  cfg.Exclude,
				MaxSize:  cfg.Indexing.MaxFileSizeBytes,
			}

			report := validate.Run(database, opts)

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, c := range report.Checks {
					val := c.Status
					if c.Details != "" {
						val = fmt.Sprintf("%s — %s", c.Status, c.Details)
					}
					if c.Violations > 0 {
						val = fmt.Sprintf("%s (%d violations)", val, c.Violations)
					}
					kvs = append(kvs, output.KV{Key: c.Name, Value: val})
				}
				status := "VALID"
				if !report.Valid {
					status = "INVALID"
				}
				kvs = append(kvs, output.KV{Key: "Overall", Value: status})
				if err := f.WriteText(kvs); err != nil {
					return err
				}
				if !report.Valid {
					return fmt.Errorf("validation failed")
				}
				return nil
			}
			if err := f.Write(report); err != nil {
				return err
			}
			if !report.Valid {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Check that all files matched by include globs exist in the index")
	return cmd
}
