package store

import (
	"database/sql"
	"time"

	"github.com/dshills/atlas/internal/extractor"
)

// UpsertReferences deletes existing references from a file and inserts new ones.
// Uses a prepared statement so that large reference lists don't re-parse SQL.
func (s *Store) UpsertReferences(tx Execer, fileID int64, refs []extractor.ReferenceRecord) (int, error) {
	if _, err := tx.Exec(`DELETE FROM "references" WHERE from_file_id = ?`, fileID); err != nil {
		return 0, err
	}

	if len(refs) == 0 {
		return 0, nil
	}

	stmt, err := tx.Prepare(`INSERT INTO "references" (from_file_id, reference_kind, confidence, line, column_start, column_end, raw_target_text, is_resolved, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0

	for _, ref := range refs {
		var line, colStart, colEnd sql.NullInt64
		if ref.Line > 0 {
			line = sql.NullInt64{Int64: int64(ref.Line), Valid: true}
		}
		if ref.ColumnStart > 0 {
			colStart = sql.NullInt64{Int64: int64(ref.ColumnStart), Valid: true}
		}
		if ref.ColumnEnd > 0 {
			colEnd = sql.NullInt64{Int64: int64(ref.ColumnEnd), Valid: true}
		}

		var rawTarget sql.NullString
		if ref.RawTargetText != "" {
			rawTarget = sql.NullString{String: ref.RawTargetText, Valid: true}
		}

		if _, err := stmt.Exec(fileID, ref.ReferenceKind, ref.Confidence, line, colStart, colEnd, rawTarget, now); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// DeleteReferencesByFile removes all outgoing references from a file.
func (s *Store) DeleteReferencesByFile(fileID int64) error {
	_, err := s.DB.Exec(`DELETE FROM "references" WHERE from_file_id = ?`, fileID)
	return err
}
