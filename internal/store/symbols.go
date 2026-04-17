package store

import (
	"database/sql"
	"time"

	"github.com/dshills/atlas/internal/extractor"
)

// UpsertSymbols deletes existing symbols for a file and inserts new ones.
// Uses a prepared statement; when called inside a transaction the per-row
// INSERT cost drops substantially.
func (s *Store) UpsertSymbols(tx Execer, fileID, packageID int64, symbols []extractor.SymbolRecord) (int, error) {
	if _, err := tx.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID); err != nil {
		return 0, err
	}

	if len(symbols) == 0 {
		return 0, nil
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO symbols (file_id, package_id, name, qualified_name, symbol_kind, visibility, parent_symbol_id, signature, doc_comment, start_line, end_line, stable_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0

	parentMap := make(map[string]int64, len(symbols))

	for _, sym := range symbols {
		var parentID sql.NullInt64
		if sym.ParentSymbolID != "" {
			if pid, ok := parentMap[sym.ParentSymbolID]; ok {
				parentID = sql.NullInt64{Int64: pid, Valid: true}
			}
		}

		var pkgID sql.NullInt64
		if packageID > 0 {
			pkgID = sql.NullInt64{Int64: packageID, Valid: true}
		}

		var sig, doc sql.NullString
		if sym.Signature != "" {
			sig = sql.NullString{String: sym.Signature, Valid: true}
		}
		if sym.DocComment != "" {
			doc = sql.NullString{String: sym.DocComment, Valid: true}
		}

		var startLine, endLine sql.NullInt64
		if sym.StartLine > 0 {
			startLine = sql.NullInt64{Int64: int64(sym.StartLine), Valid: true}
		}
		if sym.EndLine > 0 {
			endLine = sql.NullInt64{Int64: int64(sym.EndLine), Valid: true}
		}

		res, err := stmt.Exec(
			fileID, pkgID, sym.Name, sym.QualifiedName, sym.SymbolKind, sym.Visibility,
			parentID, sig, doc, startLine, endLine, sym.StableID, now, now)
		if err != nil {
			return count, err
		}

		id, _ := res.LastInsertId()
		parentMap[sym.QualifiedName] = id
		count++
	}

	return count, nil
}

// DeleteSymbolsByFile removes all symbols for a given file.
func (s *Store) DeleteSymbolsByFile(fileID int64) error {
	_, err := s.DB.Exec(`DELETE FROM symbols WHERE file_id = ?`, fileID)
	return err
}
