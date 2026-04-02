package cli

import (
	"fmt"
	"os"

	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/query"
	"github.com/spf13/cobra"
)

// StaleCmd returns the "stale" command showing stale summaries.
func StaleCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "stale",
		Short: "List stale summaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.FindStaleSummaries(db)
			if err != nil {
				return fmt.Errorf("querying stale summaries: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No stale summaries found.")
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("[%s] %s", r.Kind, r.EntityName),
						Value: fmt.Sprintf("stored=%s current=%s", r.StoredHash, r.CurrentHash),
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}
}
