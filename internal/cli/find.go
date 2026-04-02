package cli

import (
	"fmt"
	"os"

	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/query"
	"github.com/spf13/cobra"
)

// FindCmd returns the parent "find" command with subcommands for symbol, file, package, route, config.
func FindCmd(ctx *CLIContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Query symbols, files, packages, and artifacts",
	}
	cmd.AddCommand(
		findSymbolCmd(ctx),
		findFileCmd(ctx),
		findPackageCmd(ctx),
		findRouteCmd(ctx),
		findConfigCmd(ctx),
	)
	return cmd
}

func findSymbolCmd(ctx *CLIContext) *cobra.Command {
	var (
		fuzzy      bool
		kind       string
		pkg        string
		file       string
		language   string
		visibility string
	)

	cmd := &cobra.Command{
		Use:   "symbol <name>",
		Short: "Find symbols by name or qualified name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.FindSymbol(db, args[0], query.SymbolOptions{
				Fuzzy:      fuzzy,
				Kind:       kind,
				Package:    pkg,
				File:       file,
				Language:   language,
				Visibility: visibility,
			})
			if err != nil {
				return fmt.Errorf("querying symbols: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No symbols found.")
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					loc := r.FilePath
					if r.StartLine != nil {
						loc += fmt.Sprintf(":%d", *r.StartLine)
					}
					kvs = append(kvs, output.KV{
						Key:   fmt.Sprintf("%s (%s, %s)", r.QualifiedName, r.SymbolKind, r.Visibility),
						Value: loc,
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}

	cmd.Flags().BoolVar(&fuzzy, "fuzzy", false, "Enable case-insensitive substring matching")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by symbol kind")
	cmd.Flags().StringVar(&pkg, "package", "", "Filter by package name or import path")
	cmd.Flags().StringVar(&file, "file", "", "Filter by file path substring")
	cmd.Flags().StringVar(&language, "language", "", "Filter by language")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Filter by visibility (exported/unexported)")
	return cmd
}

func findFileCmd(ctx *CLIContext) *cobra.Command {
	var (
		exact   bool
		include []string
		exclude []string
	)

	cmd := &cobra.Command{
		Use:   "file <pattern>",
		Short: "Find files by path pattern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.FindFile(db, args[0], exact, query.FileOptions{
				Include: include,
				Exclude: exclude,
			})
			if err != nil {
				return fmt.Errorf("querying files: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No files found.")
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					kvs = append(kvs, output.KV{
						Key:   r.Path,
						Value: fmt.Sprintf("%s, %d bytes, %s", r.Language, r.SizeBytes, r.ParseStatus),
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}

	cmd.Flags().BoolVar(&exact, "exact", false, "Require exact path match")
	cmd.Flags().StringSliceVar(&include, "include", nil, "Glob patterns to include")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "Glob patterns to exclude")
	return cmd
}

func findPackageCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "package <name>",
		Short: "Find packages by name or import path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.FindPackage(db, args[0])
			if err != nil {
				return fmt.Errorf("querying packages: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(os.Stderr, "No packages found.")
				return nil
			}

			f := ctx.Formatter()
			if ctx.OutputMode() == output.ModeText {
				var kvs []output.KV
				for _, r := range results {
					kvs = append(kvs, output.KV{
						Key:   r.Name,
						Value: fmt.Sprintf("path=%s files=%d symbols=%d", r.DirectoryPath, r.FileCount, r.SymbolCount),
					})
				}
				return f.WriteText(kvs)
			}
			return f.Write(results)
		},
	}
}

func findRouteCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "route <pattern>",
		Short: "Find route artifacts by name pattern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return findArtifactHelper(ctx, "route", args[0])
		},
	}
}

func findConfigCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "config <key>",
		Short: "Find config key artifacts by name pattern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return findArtifactHelper(ctx, "config_key", args[0])
		},
	}
}

func findArtifactHelper(ctx *CLIContext, kind, pattern string) error {
	db, _, cleanup, err := ctx.OpenDB()
	if err != nil {
		return err
	}
	defer cleanup()

	results, err := query.FindArtifacts(db, kind, pattern)
	if err != nil {
		return fmt.Errorf("querying artifacts: %w", err)
	}

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No artifacts found.")
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
}
