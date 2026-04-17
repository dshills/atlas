package indexer

import (
	"database/sql"
	"fmt"

	"github.com/dshills/atlas/internal/diag"
	"github.com/dshills/atlas/internal/store"
)

// invalidateFile implements the 8-step cascade from Section 12.3 for a changed file.
// Runs against the provided Execer so the caller can batch it into the same
// transaction as the subsequent re-extraction.
func (idx *Indexer) invalidateFile(tx store.Execer, fileID int64) error {
	// Step 1: file metadata updated by caller
	// Step 2: delete file-owned symbols (cascades to symbol_summaries via ON DELETE CASCADE)
	if _, err := tx.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("deleting symbols: %w", err)
	}

	// Step 3: delete file-owned (outgoing) references
	if _, err := tx.Exec(`DELETE FROM "references" WHERE from_file_id = ?`, fileID); err != nil {
		return fmt.Errorf("deleting outgoing references: %w", err)
	}

	// Step 4: ON DELETE SET NULL nullifies to_symbol_id on references from other
	// files that pointed at this file's deleted symbols. to_file_id still
	// identifies those references, so narrow the update instead of scanning
	// the whole references table.
	if _, err := tx.Exec(`UPDATE "references" SET is_resolved = 0 WHERE to_file_id = ? AND to_symbol_id IS NULL AND is_resolved = 1`, fileID); err != nil {
		return fmt.Errorf("marking unresolved references: %w", err)
	}

	// Step 5: delete file summaries
	if _, err := tx.Exec(`DELETE FROM file_summaries WHERE file_id = ?`, fileID); err != nil {
		return fmt.Errorf("deleting file summaries: %w", err)
	}

	// Step 6: delete package summaries for packages containing this file
	if _, err := tx.Exec(`DELETE FROM package_summaries WHERE package_id IN (SELECT package_id FROM package_files WHERE file_id = ?)`, fileID); err != nil {
		return fmt.Errorf("deleting package summaries: %w", err)
	}

	// Step 7 & 8: re-extract and re-resolve handled by the caller after this function

	return nil
}

// resolveReferences attempts to resolve all unresolved references by matching raw_target_text against qualified_names.
func (idx *Indexer) resolveReferences() error {
	rows, err := idx.Store.DB.Query(`SELECT r.id, r.raw_target_text FROM "references" r WHERE r.is_resolved = 0 AND r.raw_target_text IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("querying unresolved references: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type unresolvedRef struct {
		id        int64
		rawTarget string
	}
	var unresolved []unresolvedRef

	for rows.Next() {
		var ref unresolvedRef
		if err := rows.Scan(&ref.id, &ref.rawTarget); err != nil {
			return err
		}
		unresolved = append(unresolved, ref)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(unresolved) == 0 {
		return nil
	}

	tx, err := idx.Store.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	lookupStmt, err := tx.Prepare(`SELECT s.id, s.file_id FROM symbols s WHERE s.qualified_name = ?`)
	if err != nil {
		return fmt.Errorf("prepare lookup: %w", err)
	}
	defer func() { _ = lookupStmt.Close() }()

	updateStmt, err := tx.Prepare(`UPDATE "references" SET to_symbol_id = ?, to_file_id = ?, is_resolved = 1 WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare update: %w", err)
	}
	defer func() { _ = updateStmt.Close() }()

	for _, ref := range unresolved {
		var symbolID int64
		var fileID int64
		err := lookupStmt.QueryRow(ref.rawTarget).Scan(&symbolID, &fileID)
		if err == sql.ErrNoRows {
			continue // still unresolved
		}
		if err != nil {
			idx.Diag.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     diag.CodeOrphanedReference,
				Message:  fmt.Sprintf("error resolving reference %q: %v", ref.rawTarget, err),
			})
			continue
		}

		if _, err := updateStmt.Exec(symbolID, fileID, ref.id); err != nil {
			idx.Diag.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     diag.CodeOrphanedReference,
				Message:  fmt.Sprintf("error updating resolved reference: %v", err),
			})
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}
