package store

import (
	"database/sql"
	"time"

	"github.com/dshills/atlas/internal/diag"
)

// PersistDiagnostics writes diagnostics to the database for a given run.
func (s *Store) PersistDiagnostics(runID int64, diagnostics []diag.Diagnostic) error {
	now := time.Now().UTC().Format(time.RFC3339)

	for _, d := range diagnostics {
		var fileID sql.NullInt64
		if d.FileID > 0 {
			fileID = sql.NullInt64{Int64: d.FileID, Valid: true}
		}
		var line, colStart, colEnd sql.NullInt64
		if d.Line > 0 {
			line = sql.NullInt64{Int64: int64(d.Line), Valid: true}
		}
		if d.ColumnStart > 0 {
			colStart = sql.NullInt64{Int64: int64(d.ColumnStart), Valid: true}
		}
		if d.ColumnEnd > 0 {
			colEnd = sql.NullInt64{Int64: int64(d.ColumnEnd), Valid: true}
		}
		var details sql.NullString
		if d.DetailsJSON != "" {
			details = sql.NullString{String: d.DetailsJSON, Valid: true}
		}

		_, err := s.DB.Exec(`INSERT INTO diagnostics (run_id, file_id, severity, code, message, line, column_start, column_end, details_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, fileID, d.Severity, d.Code, d.Message, line, colStart, colEnd, details, now)
		if err != nil {
			return err
		}
	}
	return nil
}
