package export

import (
	"database/sql"
	"fmt"
)

// SymbolExport represents an exported symbol.
type SymbolExport struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	Kind          string `json:"kind"`
	Visibility    string `json:"visibility"`
	File          string `json:"file"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	StableID      string `json:"stable_id"`
}

// ExportSymbols produces all symbols from the database.
func ExportSymbols(database *sql.DB) ([]SymbolExport, error) {
	rows, err := database.Query(`SELECT s.name, s.qualified_name, s.symbol_kind, s.visibility, f.path,
		COALESCE(s.start_line, 0), COALESCE(s.end_line, 0), s.stable_id
		FROM symbols s JOIN files f ON s.file_id = f.id ORDER BY f.path, s.start_line`)
	if err != nil {
		return nil, fmt.Errorf("querying symbols: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := []SymbolExport{}
	for rows.Next() {
		var s SymbolExport
		if err := rows.Scan(&s.Name, &s.QualifiedName, &s.Kind, &s.Visibility, &s.File, &s.StartLine, &s.EndLine, &s.StableID); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
