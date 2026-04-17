package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/diag"
	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/fswalk"
	"github.com/dshills/atlas/internal/store"
	"github.com/dshills/atlas/internal/summary"
	"github.com/dshills/atlas/internal/vcs"
)

// Indexer orchestrates file scanning, hashing, extraction, and persistence.
type Indexer struct {
	RepoRoot         string
	Config           config.Config
	Store            *store.Store
	Registry         *extractor.Registry
	Diag             *diag.Collector
	ModulePath       string
	GeneratorVersion string
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
	RunID             int64  `json:"run_id"`
	FilesScanned      int    `json:"files_scanned"`
	FilesChanged      int    `json:"files_changed"`
	FilesNew          int    `json:"files_new"`
	FilesDeleted      int    `json:"files_deleted"`
	SymbolsWritten    int    `json:"symbols_written"`
	ReferencesWritten int    `json:"references_written"`
	Status            string `json:"status"`
}

// Run executes a full or incremental index.
func (idx *Indexer) Run(mode string, since string) (*RunResult, error) {
	gitCommit := ""
	if gc, err := vcs.HeadCommit(idx.RepoRoot); err == nil {
		gitCommit = gc
	}

	runID, err := idx.Store.InsertRun(mode, gitCommit)
	if err != nil {
		return nil, fmt.Errorf("inserting run: %w", err)
	}

	result := &RunResult{RunID: runID, Status: "success"}

	candidates, err := idx.collectCandidates(since)
	if err != nil {
		return nil, err
	}

	result.FilesScanned = len(candidates)

	existingHashes, err := idx.Store.FileHashMap()
	if err != nil {
		return nil, fmt.Errorf("loading file hashes: %w", err)
	}

	existingPaths, err := idx.Store.AllFilePaths()
	if err != nil {
		return nil, fmt.Errorf("loading file paths: %w", err)
	}

	seenPaths := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		seenPaths[c.Path] = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := idx.runParseWorkers(ctx, candidates, existingHashes)

	for out := range results {
		if out.readErr != nil {
			idx.Diag.Add(diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     diag.CodeFileMissing,
				Message:  fmt.Sprintf("cannot read file %s: %v", out.candidate.Path, out.readErr),
			})
			continue
		}
		if out.skipped {
			continue
		}

		result.FilesChanged++
		if _, exists := existingHashes[out.candidate.Path]; !exists {
			result.FilesNew++
		}

		symCount, refCount := idx.persistParsed(out, gitCommit, existingHashes, existingPaths)
		result.SymbolsWritten += symCount
		result.ReferencesWritten += refCount
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

	// Cross-file reference resolution (Step 8)
	if err := idx.resolveReferences(); err != nil {
		idx.Diag.AddError(diag.CodeOrphanedReference, fmt.Sprintf("reference resolution failed: %v", err))
	}

	// Generate summaries if enabled
	if idx.Config.Summaries.Enabled {
		idx.generateSummaries()
	}

	if err := idx.Store.PersistDiagnostics(runID, idx.Diag.All()); err != nil {
		return nil, fmt.Errorf("persisting diagnostics: %w", err)
	}

	if idx.Diag.HasErrors() {
		result.Status = "partial"
	}

	run := &store.RunRow{
		ID:                runID,
		Status:            result.Status,
		FilesScanned:      result.FilesScanned,
		FilesChanged:      result.FilesChanged,
		SymbolsWritten:    result.SymbolsWritten,
		ReferencesWritten: result.ReferencesWritten,
		ErrorCount:        idx.Diag.ErrorCount(),
		WarningCount:      idx.Diag.WarningCount(),
	}
	if err := idx.Store.FinishRun(run); err != nil {
		return nil, fmt.Errorf("finishing run: %w", err)
	}

	return result, nil
}

// persistParsed is the serial writer half of the pipeline: it consumes a
// parseOutcome produced by a worker and writes the file graph to SQLite
// inside one transaction. Wrapping in a tx turns N fsyncs into 1 and is
// the single biggest perf win for indexing.
func (idx *Indexer) persistParsed(out parseOutcome, gitCommit string, existingHashes map[string]string, existingPaths map[string]int64) (int, int) {
	c := out.candidate

	tx, err := idx.Store.DB.Begin()
	if err != nil {
		idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("begin tx for %s: %v", c.Path, err))
		return 0, 0
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, exists := existingHashes[c.Path]; exists {
		if existingID, ok := existingPaths[c.Path]; ok {
			if err := idx.invalidateFile(tx, existingID); err != nil {
				idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("invalidation failed for %s: %v", c.Path, err))
			}
		}
	}

	modTime := time.Unix(c.ModTime, 0).UTC().Format(time.RFC3339)
	fileRow := &store.FileRow{
		Path:            c.Path,
		Language:        c.Language,
		ContentHash:     out.contentHash,
		SizeBytes:       c.Size,
		LastModifiedUTC: sql.NullString{String: modTime, Valid: true},
		GitCommit:       sql.NullString{String: gitCommit, Valid: gitCommit != ""},
		IsGenerated:     c.IsGenerated,
		ParseStatus:     "skipped",
	}

	fileID, err := idx.Store.UpsertFile(tx, fileRow)
	if err != nil {
		idx.Diag.Add(diag.Diagnostic{
			Severity: diag.SeverityError,
			Code:     diag.CodeParseError,
			Message:  fmt.Sprintf("failed to persist file %s: %v", c.Path, err),
		})
		return 0, 0
	}

	if !out.hasExtractor {
		idx.Diag.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     diag.CodeUnsupportedLang,
			Message:  fmt.Sprintf("no extractor for %s", c.Path),
		})
		if err := tx.Commit(); err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("commit tx for %s: %v", c.Path, err))
			return 0, 0
		}
		committed = true
		return 0, 0
	}

	if out.extractErr != nil {
		idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("extraction failed for %s: %v", c.Path, out.extractErr))
		if err := idx.Store.SetParseStatus(tx, fileID, "error"); err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to update parse_status for %s: %v", c.Path, err))
		}
		if err := tx.Commit(); err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("commit tx for %s: %v", c.Path, err))
			return 0, 0
		}
		committed = true
		return 0, 0
	}

	symCount, refCount, parseStatus := idx.persistExtractResult(tx, c, fileID, out.res)
	if parseStatus != "" {
		if err := idx.Store.SetParseStatus(tx, fileID, parseStatus); err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to update parse_status for %s: %v", c.Path, err))
		}
	}

	if err := tx.Commit(); err != nil {
		idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("commit tx for %s: %v", c.Path, err))
		return 0, 0
	}
	committed = true
	return symCount, refCount
}

// persistExtractResult writes the symbols, references, artifacts, and
// package produced by a worker's extractor call into the given tx. It is
// the DB-bound counterpart to parseOne.
func (idx *Indexer) persistExtractResult(tx store.Execer, c fswalk.FileCandidate, fileID int64, res *extractor.ExtractResult) (int, int, string) {
	// Record diagnostics from extraction
	for _, d := range res.Diagnostics {
		idx.Diag.Add(diag.Diagnostic{
			Severity: d.Severity,
			Code:     d.Code,
			Message:  d.Message,
			FileID:   fileID,
			Line:     d.Line,
		})
	}

	parseStatus := ""
	if res.File != nil {
		parseStatus = res.File.ParseStatus
	}

	// Persist package
	var packageID int64
	if res.Package != nil {
		pkgRow := &store.PackageRow{
			Name:          res.Package.Name,
			ImportPath:    sql.NullString{String: res.Package.ImportPath, Valid: res.Package.ImportPath != ""},
			DirectoryPath: res.Package.DirectoryPath,
			Language:      res.Package.Language,
		}
		pid, err := idx.Store.UpsertPackage(tx, pkgRow)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist package for %s: %v", c.Path, err))
		} else {
			packageID = pid
			_ = idx.Store.LinkFileToPackage(tx, fileID, packageID)

			// Update denormalized fields
			if _, err := tx.Exec(`UPDATE files SET package_name = ?, module_name = ? WHERE id = ?`,
				res.Package.Name, sql.NullString{String: res.Package.ImportPath, Valid: res.Package.ImportPath != ""}, fileID); err != nil {
				idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to update denormalized fields for %s: %v", c.Path, err))
			}
		}
	}

	// Persist symbols
	symCount := 0
	if len(res.Symbols) > 0 {
		n, err := idx.Store.UpsertSymbols(tx, fileID, packageID, res.Symbols)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist symbols for %s: %v", c.Path, err))
		}
		symCount = n
	}

	// Persist references
	refCount := 0
	if len(res.References) > 0 {
		n, err := idx.Store.UpsertReferences(tx, fileID, res.References)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist references for %s: %v", c.Path, err))
		}
		refCount = n
	}

	// Persist artifacts
	if len(res.Artifacts) > 0 {
		if _, err := idx.Store.UpsertArtifacts(tx, fileID, res.Artifacts); err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist artifacts for %s: %v", c.Path, err))
		}
	}

	return symCount, refCount, parseStatus
}

func (idx *Indexer) collectCandidates(since string) ([]fswalk.FileCandidate, error) {
	if since != "" {
		diffFiles, err := vcs.DiffFiles(idx.RepoRoot, since)
		if err != nil {
			return nil, fmt.Errorf("git diff: %w", err)
		}
		candidates := make([]fswalk.FileCandidate, 0, len(diffFiles))
		for _, rel := range diffFiles {
			c, ok := fswalk.StatCandidate(idx.RepoRoot, rel, idx.Config.Include, idx.Config.Exclude, idx.Config.Indexing.MaxFileSizeBytes)
			if !ok {
				continue
			}
			candidates = append(candidates, c)
		}
		return candidates, nil
	}

	candidates, err := fswalk.Walk(idx.RepoRoot, idx.Config.Include, idx.Config.Exclude, idx.Config.Indexing.MaxFileSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("walking: %w", err)
	}
	return candidates, nil
}

// generateSummaries creates summaries for all indexed files and packages.
func (idx *Indexer) generateSummaries() {
	gen := summary.NewGenerator(idx.Store.DB, idx.GeneratorVersion)

	if idx.Config.Summaries.File || idx.Config.Summaries.Symbol {
		rows, err := idx.Store.DB.Query(`SELECT id FROM files WHERE parse_status = 'ok'`)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("querying files for summaries: %v", err))
			return
		}
		var fileIDs []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("scanning file ID: %v", err))
				return
			}
			fileIDs = append(fileIDs, id)
		}
		_ = rows.Close()

		if idx.Config.Summaries.File {
			for _, fid := range fileIDs {
				if err := gen.GenerateFileSummary(fid); err != nil {
					idx.Diag.AddWarning(diag.CodeParseError, fmt.Sprintf("file summary generation failed: %v", err))
				}
			}
		}

		if idx.Config.Summaries.Symbol && len(fileIDs) > 0 {
			symIDs, err := idx.loadSymbolIDsForFiles(fileIDs)
			if err != nil {
				idx.Diag.AddWarning(diag.CodeParseError, fmt.Sprintf("querying symbols for summaries: %v", err))
			}
			for _, sid := range symIDs {
				if err := gen.GenerateSymbolSummary(sid); err != nil {
					idx.Diag.AddWarning(diag.CodeParseError, fmt.Sprintf("symbol summary generation failed: %v", err))
				}
			}
		}
	}

	if idx.Config.Summaries.Package {
		if err := gen.GenerateAllPackages(); err != nil {
			idx.Diag.AddWarning(diag.CodeParseError, fmt.Sprintf("package summary generation failed: %v", err))
		}
	}
}

// loadSymbolIDsForFiles returns all symbol IDs belonging to the given file IDs
// in a single query, replacing an N+1 loop of per-file SELECTs. SQLite
// parameter limits (999 by default) force us to chunk large ID lists.
func (idx *Indexer) loadSymbolIDsForFiles(fileIDs []int64) ([]int64, error) {
	const chunk = 500
	symIDs := make([]int64, 0, len(fileIDs)*4)

	for start := 0; start < len(fileIDs); start += chunk {
		end := min(start+chunk, len(fileIDs))
		batch := fileIDs[start:end]

		placeholders := make([]byte, 0, len(batch)*2)
		args := make([]any, 0, len(batch))
		for i, id := range batch {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, '?')
			args = append(args, id)
		}

		query := `SELECT id FROM symbols WHERE file_id IN (` + string(placeholders) + `)`
		rows, err := idx.Store.DB.Query(query, args...)
		if err != nil {
			return symIDs, err
		}
		for rows.Next() {
			var sid int64
			if err := rows.Scan(&sid); err != nil {
				_ = rows.Close()
				return symIDs, err
			}
			symIDs = append(symIDs, sid)
		}
		if err := rows.Close(); err != nil {
			return symIDs, err
		}
	}

	return symIDs, nil
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
