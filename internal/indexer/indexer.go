package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/diag"
	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/fswalk"
	"github.com/dshills/atlas/internal/hash"
	"github.com/dshills/atlas/internal/store"
	"github.com/dshills/atlas/internal/vcs"
)

// Indexer orchestrates file scanning, hashing, extraction, and persistence.
type Indexer struct {
	RepoRoot   string
	Config     config.Config
	Store      *store.Store
	Registry   *extractor.Registry
	Diag       *diag.Collector
	ModulePath string
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
			continue
		}

		result.FilesChanged++
		if _, exists := existingHashes[c.Path]; !exists {
			result.FilesNew++
		}

		modTime := time.Unix(c.ModTime, 0).UTC().Format(time.RFC3339)
		parseStatus := "skipped"

		fileRow := &store.FileRow{
			Path:            c.Path,
			Language:        c.Language,
			ContentHash:     contentHash,
			SizeBytes:       c.Size,
			LastModifiedUTC: sql.NullString{String: modTime, Valid: true},
			GitCommit:       sql.NullString{String: gitCommit, Valid: gitCommit != ""},
			IsGenerated:     c.IsGenerated,
			ParseStatus:     parseStatus,
		}

		fileID, err := idx.Store.UpsertFile(fileRow)
		if err != nil {
			idx.Diag.Add(diag.Diagnostic{
				Severity: diag.SeverityError,
				Code:     diag.CodeParseError,
				Message:  fmt.Sprintf("failed to persist file %s: %v", c.Path, err),
			})
			continue
		}

		// Run extractor if available
		if idx.Registry != nil {
			ext, extErr := idx.Registry.ForPath(c.Path)
			if extErr != nil {
				idx.Diag.Add(diag.Diagnostic{
					Severity: diag.SeverityInfo,
					Code:     diag.CodeUnsupportedLang,
					Message:  fmt.Sprintf("no extractor for %s", c.Path),
				})
				continue
			}

			symCount, refCount, extractParseStatus := idx.extractAndPersist(ext, c, content, fileID)
			result.SymbolsWritten += symCount
			result.ReferencesWritten += refCount

			if extractParseStatus != "" {
				if _, err := idx.Store.DB.Exec(`UPDATE files SET parse_status = ? WHERE id = ?`, extractParseStatus, fileID); err != nil {
					idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to update parse_status for %s: %v", c.Path, err))
				}
			}
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

func (idx *Indexer) extractAndPersist(ext extractor.Extractor, c fswalk.FileCandidate, content []byte, fileID int64) (int, int, string) {
	req := extractor.ExtractRequest{
		FilePath:   c.Path,
		AbsPath:    c.AbsPath,
		Content:    content,
		RepoRoot:   idx.RepoRoot,
		ModulePath: idx.ModulePath,
	}

	res, err := ext.Extract(context.Background(), req)
	if err != nil {
		idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("extraction failed for %s: %v", c.Path, err))
		return 0, 0, "error"
	}

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
		pid, err := idx.Store.UpsertPackage(pkgRow)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist package for %s: %v", c.Path, err))
		} else {
			packageID = pid
			_ = idx.Store.LinkFileToPackage(fileID, packageID)

			// Update denormalized fields
			if _, err := idx.Store.DB.Exec(`UPDATE files SET package_name = ?, module_name = ? WHERE id = ?`,
				res.Package.Name, sql.NullString{String: res.Package.ImportPath, Valid: res.Package.ImportPath != ""}, fileID); err != nil {
				idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to update denormalized fields for %s: %v", c.Path, err))
			}
		}
	}

	// Persist symbols
	symCount := 0
	if len(res.Symbols) > 0 {
		n, err := idx.Store.UpsertSymbols(fileID, packageID, res.Symbols)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist symbols for %s: %v", c.Path, err))
		}
		symCount = n
	}

	// Persist references
	refCount := 0
	if len(res.References) > 0 {
		n, err := idx.Store.UpsertReferences(fileID, res.References)
		if err != nil {
			idx.Diag.AddError(diag.CodeParseError, fmt.Sprintf("failed to persist references for %s: %v", c.Path, err))
		}
		refCount = n
	}

	// Persist artifacts
	if len(res.Artifacts) > 0 {
		if _, err := idx.Store.UpsertArtifacts(fileID, res.Artifacts); err != nil {
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
		allCandidates, err := fswalk.Walk(idx.RepoRoot, idx.Config.Include, idx.Config.Exclude, idx.Config.Indexing.MaxFileSizeBytes)
		if err != nil {
			return nil, fmt.Errorf("walking: %w", err)
		}
		diffSet := make(map[string]bool, len(diffFiles))
		for _, f := range diffFiles {
			diffSet[f] = true
		}
		var candidates []fswalk.FileCandidate
		for _, c := range allCandidates {
			if diffSet[c.Path] {
				candidates = append(candidates, c)
			}
		}
		return candidates, nil
	}

	candidates, err := fswalk.Walk(idx.RepoRoot, idx.Config.Include, idx.Config.Exclude, idx.Config.Indexing.MaxFileSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("walking: %w", err)
	}
	return candidates, nil
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
