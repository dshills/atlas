package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/query"
	"github.com/spf13/cobra"
)

// WhoCallsCmd returns the "who-calls" command.
func WhoCallsCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "who-calls <symbol>",
		Short: "Find incoming callers of a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return relationshipHelper(ctx, "who-calls", args[0], query.WhoCalls)
		},
	}
}

// CallsCmd returns the "calls" command.
func CallsCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "calls <symbol>",
		Short: "Find outgoing calls from a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return relationshipHelper(ctx, "calls", args[0], query.Calls)
		},
	}
}

// ImplementationsCmd returns the "implementations" command.
func ImplementationsCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "implementations <interface>",
		Short: "Find implementations of an interface",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return relationshipHelper(ctx, "implementations", args[0], query.Implementations)
		},
	}
}

// ImportsCmd returns the "imports" command.
func ImportsCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "imports <package>",
		Short: "Find references that import a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return relationshipHelper(ctx, "imports", args[0], query.Imports)
		},
	}
}

// TestsForCmd returns the "tests-for" command.
func TestsForCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "tests-for <target>",
		Short: "Find tests for a symbol or function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return relationshipHelper(ctx, "tests-for", args[0], query.TestsFor)
		},
	}
}

// TouchesCmd returns the "touches" command.
func TouchesCmd(ctx *CLIContext) *cobra.Command {
	return &cobra.Command{
		Use:   "touches <artifact-kind> <name>",
		Short: "Find references touching an artifact",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, _, cleanup, err := ctx.OpenDB()
			if err != nil {
				return err
			}
			defer cleanup()

			results, err := query.Touches(db, args[0], args[1])
			if err != nil {
				return fmt.Errorf("querying touches: %w", err)
			}

			return writeRelationships(ctx, results)
		},
	}
}

func relationshipHelper(ctx *CLIContext, cmdName, symbolName string, queryFn func(*sql.DB, string) ([]query.RelationshipResult, error)) error {
	db, _, cleanup, err := ctx.OpenDB()
	if err != nil {
		return err
	}
	defer cleanup()

	results, err := queryFn(db, symbolName)
	if err != nil {
		return fmt.Errorf("querying %s: %w", cmdName, err)
	}

	return writeRelationships(ctx, results)
}

func writeRelationships(ctx *CLIContext, results []query.RelationshipResult) error {
	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No relationships found.")
		return nil
	}

	f := ctx.Formatter()
	if ctx.OutputMode() == output.ModeText {
		var kvs []output.KV
		for _, r := range results {
			loc := r.FromFile
			if r.Line != nil {
				loc += fmt.Sprintf(":%d", *r.Line)
			}
			label := fmt.Sprintf("%s -> %s", r.FromSymbol, r.ToSymbol)
			if r.FromSymbol == "" && r.RawTargetText != "" {
				label = r.RawTargetText
			}
			kvs = append(kvs, output.KV{
				Key:   label,
				Value: fmt.Sprintf("%s (%s, %s)", loc, r.ReferenceKind, r.Confidence),
			})
		}
		return f.WriteText(kvs)
	}
	return f.Write(results)
}
