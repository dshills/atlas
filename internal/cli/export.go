package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dshills/atlas/internal/export"
	"github.com/spf13/cobra"
)

// ExportCmd creates the `atlas export` command with subcommands.
func ExportCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export index data as stable JSON",
		Long: `Export index data as stable JSON. All subcommands support --out <file>.

Subcommands:
  summary      Repository overview (packages, entrypoints, stale counts)
  graph        Symbol/reference graph as nodes and edges
  symbols      All symbols with qualified names, kinds, locations
  packages     All packages with file counts
  routes       All route artifacts
  diagnostics  Diagnostics from the latest run`,
		Example: `  atlas export summary --agent
  atlas export graph --out graph.json
  atlas export symbols`,
	}

	cmd.AddCommand(
		exportSummaryCmd(ctx),
		exportGraphCmd(ctx),
		exportSymbolsCmd(ctx),
		exportPackagesCmd(ctx),
		exportRoutesCmd(ctx),
		exportDiagnosticsCmd(ctx),
	)

	return cmd
}

func writeExport(cmd *cobra.Command, data any) error {
	outFile, _ := cmd.Flags().GetString("out")

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	encoded = append(encoded, '\n')

	if outFile != "" {
		return os.WriteFile(outFile, encoded, 0o644)
	}
	_, err = os.Stdout.Write(encoded)
	return err
}

func exportSummaryCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Export repository summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := ctx.RepoRoot()
			if err != nil {
				return err
			}
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			data, err := export.ExportSummary(database, repoRoot)
			if err != nil {
				return err
			}
			return writeExport(cmd, data)
		},
	}
	cmd.Flags().String("out", "", "Write output to file")
	return cmd
}

func exportGraphCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Export symbol/reference graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			data, err := export.ExportGraph(database)
			if err != nil {
				return err
			}
			return writeExport(cmd, data)
		},
	}
	cmd.Flags().String("out", "", "Write output to file")
	return cmd
}

func exportSymbolsCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "symbols",
		Short: "Export all symbols",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			data, err := export.ExportSymbols(database)
			if err != nil {
				return err
			}
			return writeExport(cmd, data)
		},
	}
	cmd.Flags().String("out", "", "Write output to file")
	return cmd
}

func exportPackagesCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Export all packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			data, err := export.ExportPackages(database)
			if err != nil {
				return err
			}
			return writeExport(cmd, data)
		},
	}
	cmd.Flags().String("out", "", "Write output to file")
	return cmd
}

func exportRoutesCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Export all routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			data, err := export.ExportRoutes(database)
			if err != nil {
				return err
			}
			return writeExport(cmd, data)
		},
	}
	cmd.Flags().String("out", "", "Write output to file")
	return cmd
}

func exportDiagnosticsCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Export diagnostics from latest run",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			data, err := export.ExportDiagnostics(database)
			if err != nil {
				return err
			}
			return writeExport(cmd, data)
		},
	}
	cmd.Flags().String("out", "", "Write output to file")
	return cmd
}
