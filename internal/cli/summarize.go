package cli

import (
	"database/sql"
	"fmt"

	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/store"
	"github.com/dshills/atlas/internal/summary"
	"github.com/spf13/cobra"
)

// SummarizeCmd creates the `atlas summarize` command with file/package/symbol subcommands.
func SummarizeCmd(ctx *CLIContext, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "Generate or retrieve summaries",
	}

	cmd.AddCommand(
		summarizeFileCmd(ctx, version),
		summarizePackageCmd(ctx, version),
		summarizeSymbolCmd(ctx, version),
	)

	return cmd
}

func summarizeFileCmd(ctx *CLIContext, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "file <path>",
		Short: "Generate/retrieve file summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			path := args[0]
			var fileID int64
			err = database.QueryRow(`SELECT id FROM files WHERE path = ?`, path).Scan(&fileID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("file not found: %s", path)
			}
			if err != nil {
				return err
			}

			gen := summary.NewGenerator(database, version)
			if err := gen.GenerateFileSummary(fileID); err != nil {
				return fmt.Errorf("generating summary: %w", err)
			}

			s := store.New(database)
			row, err := s.GetFileSummary(fileID)
			if err != nil {
				return fmt.Errorf("retrieving summary: %w", err)
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "File", Value: path},
					{Key: "Summary", Value: row.SummaryText},
					{Key: "Key symbols", Value: fmt.Sprintf("%v", row.KeySymbols)},
					{Key: "Dependencies", Value: fmt.Sprintf("%v", row.Dependencies)},
					{Key: "Public API", Value: fmt.Sprintf("%v", row.PublicAPI)},
					{Key: "Hash", Value: row.GeneratedFromHash},
				})
			}
			return f.Write(row)
		},
	}
}

func summarizePackageCmd(ctx *CLIContext, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "package <name>",
		Short: "Generate/retrieve package summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			name := args[0]
			var packageID int64
			err = database.QueryRow(`SELECT id FROM packages WHERE name = ? OR import_path = ?`, name, name).Scan(&packageID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("package not found: %s", name)
			}
			if err != nil {
				return err
			}

			gen := summary.NewGenerator(database, version)
			if err := gen.GeneratePackageSummary(packageID); err != nil {
				return fmt.Errorf("generating summary: %w", err)
			}

			s := store.New(database)
			row, err := s.GetPackageSummary(packageID)
			if err != nil {
				return fmt.Errorf("retrieving summary: %w", err)
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Package", Value: name},
					{Key: "Summary", Value: row.SummaryText},
					{Key: "Exported", Value: fmt.Sprintf("%v", row.ExportedSurface)},
					{Key: "Internal deps", Value: fmt.Sprintf("%v", row.InternalCollaborators)},
					{Key: "External deps", Value: fmt.Sprintf("%v", row.ExternalDependencies)},
					{Key: "Hash", Value: row.GeneratedFromHash},
				})
			}
			return f.Write(row)
		},
	}
}

func summarizeSymbolCmd(ctx *CLIContext, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "symbol <qualified-name>",
		Short: "Generate/retrieve symbol summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			qname := args[0]
			var symbolID int64
			err = database.QueryRow(`SELECT id FROM symbols WHERE qualified_name = ?`, qname).Scan(&symbolID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("symbol not found: %s", qname)
			}
			if err != nil {
				return err
			}

			gen := summary.NewGenerator(database, version)
			if err := gen.GenerateSymbolSummary(symbolID); err != nil {
				return fmt.Errorf("generating summary: %w", err)
			}

			s := store.New(database)
			row, err := s.GetSymbolSummary(symbolID)
			if err != nil {
				return fmt.Errorf("retrieving summary: %w", err)
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				return f.WriteText([]output.KV{
					{Key: "Symbol", Value: qname},
					{Key: "Summary", Value: row.SummaryText},
					{Key: "Intent", Value: fmt.Sprintf("%v", row.Intent)},
					{Key: "Inputs", Value: fmt.Sprintf("%v", row.Inputs)},
					{Key: "Related", Value: fmt.Sprintf("%v", row.RelatedSymbols)},
					{Key: "Hash", Value: row.GeneratedFromHash},
				})
			}
			return f.Write(row)
		},
	}
}
