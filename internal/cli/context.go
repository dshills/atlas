// Package cli provides Cobra commands for Atlas query and reporting.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/db"
	"github.com/dshills/atlas/internal/output"
	"github.com/dshills/atlas/internal/repo"
)

// CLIContext holds shared state for CLI commands.
type CLIContext struct {
	FlagRepo  *string
	FlagJSON  *bool
	FlagAgent *bool
}

// OutputMode returns the current output mode based on flags.
func (c *CLIContext) OutputMode() output.Mode {
	if c.FlagAgent != nil && *c.FlagAgent {
		return output.ModeAgent
	}
	if c.FlagJSON != nil && *c.FlagJSON {
		return output.ModeJSON
	}
	return output.ModeText
}

// Formatter returns a new output formatter writing to stdout.
func (c *CLIContext) Formatter() *output.Formatter {
	return output.New(os.Stdout, c.OutputMode())
}

// RepoRoot resolves the repository root.
func (c *CLIContext) RepoRoot() (string, error) {
	flagVal := ""
	if c.FlagRepo != nil {
		flagVal = *c.FlagRepo
	}
	return repo.FindRoot(flagVal, "")
}

// OpenDB opens the Atlas database and returns the raw *sql.DB plus a cleanup function.
func (c *CLIContext) OpenDB() (*sql.DB, string, func(), error) {
	repoRoot, err := c.RepoRoot()
	if err != nil {
		return nil, "", nil, fmt.Errorf("finding repo root: %w", err)
	}

	dbPath := filepath.Join(repoRoot, config.DefaultStorageDir, config.DefaultDBFile)
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, "", nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.CheckSchemaVersion(database); err != nil {
		_ = database.Close()
		return nil, "", nil, err
	}
	cleanup := func() { _ = database.Close() }
	return database, dbPath, cleanup, nil
}
