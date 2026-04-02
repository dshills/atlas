package indexer

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/diag"
	"github.com/dshills/atlas/internal/fswalk"
	"github.com/dshills/atlas/internal/hash"
	"github.com/dshills/atlas/internal/store"
	"github.com/dshills/atlas/internal/vcs"
)

// Indexer orchestrates file scanning, hashing, and persistence.
type Indexer struct {
	RepoRoot string
	Config   config.Config
	Store    *store.Store
	Diag     *diag.Collector
}

// New creates a new Indexer.
func New(repoRoot string, cfg config.Config, s *store.Store) *Indexer {
	return &Indexer{
		RepoRoot: repoRoot,
		Config:   cfg,
		Store:    s,
		Diag:     diag.NewCollector(),
	}
}

// RunResult holds the results of an indexing run.
type RunResult struct {
	RunID        int64
	FilesScanned int
	FilesChanged int
	FilesNew     int
	FilesDeleted int
	Status       string
}

// Run executes a full or incremental index.
func (idx *Indexer) Run(mode string, since string) (*RunResult, error) {
	// Get git commit if available
	gitCommit := ""
	if gc, err := vcs.HeadCommit(idx.RepoRoot); err == nil {
		gitCommit = gc
	}

	runID, err := idx.Store.InsertRun(mode, gitCommit)
	if err != nil {
		return nil, fmt.Errorf("inserting run: %w", err)
	}

	result := &RunResult{RunID: runID, Status: "success"}

	// Determine which files to process
	var candidates []fswalk.FileCandidate
	if since != "" {
		// Incremental based on git diff
		diffFiles, err := vcs.DiffFiles(idx.RepoRoot, since)
		if err != nil {
			return nil, fmt.Errorf("git diff: %w", err)
		}
		// Filter diff files through walker logic
		allCandidates, err := fswalk.Walk(idx.RepoRoot, idx.Config.Include, idx.Config.Exclude, idx.Config.Indexing.MaxFileSizeBytes)
		if err != nil {
			return nil, fmt.Errorf("walking: %w", err)
		}
		diffSet := make(map[string]bool, len(diffFiles))
		for _, f := range diffFiles {
			diffSet[f] = true
		}
		for _, c := range allCandidates {
			if diffSet[c.Path] {
				candidates = append(candidates, c)
			}
		}
	} else {
		candidates, err = fswalk.Walk(idx.RepoRoot, idx.Config.Include, idx.Config.Exclude, idx.Config.Indexing.MaxFileSizeBytes)
		if err != nil {
			return nil, fmt.Errorf("walking: %w", err)
		}
	}

	result.FilesScanned = len(candidates)

	// Get existing file hashes
	existingHashes, err := idx.Store.FileHashMap()
	if err != nil {
		return nil, fmt.Errorf("loading file hashes: %w", err)
	}

	// Get existing file paths for deletion detection
	existingPaths, err := idx.Store.AllFilePaths()
	if err != nil {
		return nil, fmt.Errorf("loading file paths: %w", err)
	}

	// Track which files we've seen in this scan
	seenPaths := make(map[string]bool, len(candidates))

	// Process each candidate
	for _, c := range candidates {
		seenPaths[c.Path] = true

		content, err := os.ReadFile(c.AbsPath)
		if err != nil {
			idx.Diag.Add(diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     diag.CodeFileMissing,
				Message:  fmt.Sprintf("cannot read file %s: %v", c.Path, err),
			})
			continue
		}

		contentHash := hash.Hash(content)
		if existingHash, exists := existingHashes[c.Path]; exists && existingHash == contentHash {
			continue // unchanged
		}

		result.FilesChanged++
		if _, exists := existingHashes[c.Path]; !exists {
			result.FilesNew++
		}

		modTime := time.Unix(c.ModTime, 0).UTC().Format(time.RFC3339)
		fileRow := &store.FileRow{
			Path:            c.Path,
			Language:        c.Language,
			ContentHash:     contentHash,
			SizeBytes:       c.Size,
			LastModifiedUTC: sql.NullString{String: modTime, Valid: true},
			GitCommit:       sql.NullString{String: gitCommit, Valid: gitCommit != ""},
			IsGenerated:     c.IsGenerated,
			ParseStatus:     "skipped", // Phase 2: no parsing yet
		}

		if _, err := idx.Store.UpsertFile(fileRow); err != nil {
			idx.Diag.Add(diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     diag.CodeParseError,
				Message:  fmt.Sprintf("failed to persist file %s: %v", c.Path, err),
			})
		}
	}

	// Handle deletions (only in full mode)
	if mode == "full" {
		for path, id := range existingPaths {
			if !seenPaths[path] {
				if err := idx.Store.DeleteFile(id); err != nil {
					idx.Diag.Add(diag.Diagnostic{
						Severity: diag.SeverityWarning,
						Code:     diag.CodeFileMissing,
						Message:  fmt.Sprintf("failed to delete file %s: %v", path, err),
					})
				}
				result.FilesDeleted++
			}
		}
	}

	// Persist diagnostics
	if err := idx.Store.PersistDiagnostics(runID, idx.Diag.All()); err != nil {
		return nil, fmt.Errorf("persisting diagnostics: %w", err)
	}

	// Determine final status
	if idx.Diag.HasErrors() {
		if idx.Config.Indexing.MaxFileSizeBytes > 0 { // proxy for "strict not set"
			result.Status = "partial"
		}
	}

	// Finish run
	run := &store.RunRow{
		ID:           runID,
		Status:       result.Status,
		FilesScanned: result.FilesScanned,
		FilesChanged: result.FilesChanged,
		ErrorCount:   idx.Diag.ErrorCount(),
		WarningCount: idx.Diag.WarningCount(),
	}
	if err := idx.Store.FinishRun(run); err != nil {
		return nil, fmt.Errorf("finishing run: %w", err)
	}

	return result, nil
}

// ClearAll removes all derived data for a full reindex.
func (idx *Indexer) ClearAll() error {
	tables := []string{"diagnostics", "index_runs", "file_summaries", "package_summaries", "symbol_summaries", "artifacts", "references", "symbols", "package_files", "packages", "files"}
	for _, t := range tables {
		if _, err := idx.Store.DB.Exec(fmt.Sprintf("DELETE FROM %q", t)); err != nil {
			return fmt.Errorf("clearing %s: %w", t, err)
		}
	}
	return nil
}
