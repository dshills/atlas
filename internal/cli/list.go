package cli

import (
	"fmt"
	"os"

	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/query"
	"github.com/spf13/cobra"
)

// ListCmd returns the parent "list" command with subcommands.
func ListCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages, routes, jobs, migrations, integrations, entrypoints, or diagnostics",
		Example: `  atlas list packages
  atlas list routes --agent
  atlas list jobs
  atlas list diagnostics`,
	}
	cmd.AddCommand(
		listPackagesCmd(ctx),
		listArtifactCmd(ctx, "routes", "route", "List all registered routes"),
		listArtifactCmd(ctx, "jobs", "background_job", "List all background jobs"),
		listArtifactCmd(ctx, "migrations", "migration", "List all migrations"),
		listArtifactCmd(ctx, "integrations", "external_service", "List all external service integrations"),
		listArtifactCmd(ctx, "entrypoints", "cli_command", "List all CLI command entrypoints"),
		listDiagnosticsCmd(ctx),
	)
	return cmd
}

func listPackagesCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "packages",
		Short: "List all packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.ListPackages(db)
			if err != nil {
				return fmt.Errorf("listing packages: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No packages found.")
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					val := fmt.Sprintf("files=%d symbols=%d lang=%s", r.FileCount, r.SymbolCount, r.Language)
					if r.ImportPath != "" {
						val = fmt.Sprintf("import=%s %s", r.ImportPath, val)
					}
					kvs = append(kvs, output.KV{
						Key:   r.Name,
						Value: val,
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}
}

func listArtifactCmd(ctx *CLIContext, use, artifactKind, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.ListArtifactsByKind(db, artifactKind)
			if err != nil {
				return fmt.Errorf("listing %s: %w", use, err)
			}

			if len(results) == 0 {
				fmt.Fprintf(os.Stderr, "No %s found.\n", use)
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					kvs = append(kvs, output.KV{
						Key:   r.Name,
						Value: fmt.Sprintf("%s (%s)", r.FilePath, r.Confidence),
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}
}

func listDiagnosticsCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "diagnostics",
		Short: "List diagnostics from the latest run",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.ListDiagnostics(db)
			if err != nil {
				return fmt.Errorf("listing diagnostics: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No diagnostics found.")
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					loc := r.FilePath
					if r.Line != nil {
						loc += fmt.Sprintf(":%d", *r.Line)
					}
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("[%s] %s", r.Severity, r.Code),
						Value: fmt.Sprintf("%s — %s", loc, r.Message),
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}
}
