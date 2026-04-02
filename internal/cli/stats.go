package cli

import (
	"fmt"

	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/query"
	"github.com/spf13/cobra"
)

// StatsCmd returns the "stats" command showing repository statistics.
func StatsCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show repository statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, dbPath, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			stats, err := query.GetStats(db, dbPath)
			if err != nil {
				return fmt.Errorf("gathering stats: %w", err)
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV

				// Files by language
				totalFiles := 0
				for lang, count := range stats.FilesByLanguage {
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("Files (%s)", lang),
						Value: fmt.Sprintf("%d", count),
					})
					totalFiles += count
				}
				kvs = append(kvs, output.KV{Key: "Files (total)", Value: fmt.Sprintf("%d", totalFiles)})

				kvs = append(kvs, output.KV{Key: "Packages", Value: fmt.Sprintf("%d", stats.PackageCount)})

				// Symbols by kind
				totalSymbols := 0
				for kind, count := range stats.SymbolsByKind {
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("Symbols (%s)", kind),
						Value: fmt.Sprintf("%d", count),
					})
					totalSymbols += count
				}
				kvs = append(kvs, output.KV{Key: "Symbols (total)", Value: fmt.Sprintf("%d", totalSymbols)})

				// References by kind
				totalRefs := 0
				for kind, count := range stats.ReferencesByKind {
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("References (%s)", kind),
						Value: fmt.Sprintf("%d", count),
					})
					totalRefs += count
				}
				kvs = append(kvs, output.KV{Key: "References (total)", Value: fmt.Sprintf("%d", totalRefs)})

				// Artifacts by kind
				totalArtifacts := 0
				for kind, count := range stats.ArtifactsByKind {
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("Artifacts (%s)", kind),
						Value: fmt.Sprintf("%d", count),
					})
					totalArtifacts += count
				}
				kvs = append(kvs, output.KV{Key: "Artifacts (total)", Value: fmt.Sprintf("%d", totalArtifacts)})

				// Stale summaries
				kvs = append(kvs,
					output.KV{Key: "Stale file summaries", Value: fmt.Sprintf("%d", stats.StaleSummaries.File)},
					output.KV{Key: "Stale package summaries", Value: fmt.Sprintf("%d", stats.StaleSummaries.Package)},
					output.KV{Key: "Stale symbol summaries", Value: fmt.Sprintf("%d", stats.StaleSummaries.Symbol)},
				)

				// Last run
				if stats.LastRun != nil {
					kvs = append(kvs,
						output.KV{Key: "Last run", Value: fmt.Sprintf("%s (%s, %s)", stats.LastRun.StartedAt, stats.LastRun.Mode, stats.LastRun.Status)},
					)
				}

				// DB size
				kvs = append(kvs, output.KV{Key: "DB file size", Value: fmt.Sprintf("%d bytes", stats.DBFileSizeBytes)})

				return f.WriteText(kvs)
			}
			return f.Write(stats)
		},
	}
}
