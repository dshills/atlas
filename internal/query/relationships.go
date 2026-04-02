package query

import (
	"database/sql"
)

// RelationshipResult represents a relationship query result.
type RelationshipResult struct {
	ReferenceID   int64  `json:"referenceId"`
	FromSymbol    string `json:"fromSymbol,omitempty"`
	FromFile      string `json:"fromFile"`
	ToSymbol      string `json:"toSymbol,omitempty"`
	ToFile        string `json:"toFile,omitempty"`
	ReferenceKind string `json:"referenceKind"`
	Confidence    string `json:"confidence"`
	Line          *int64 `json:"line,omitempty"`
	ColumnStart   *int64 `json:"columnStart,omitempty"`
	ColumnEnd     *int64 `json:"columnEnd,omitempty"`
	RawTargetText string `json:"rawTargetText,omitempty"`
	IsResolved    bool   `json:"isResolved"`
}

// WhoCalls returns references where to_symbol matches the given symbol name (incoming callers).
func WhoCalls(db *sql.DB, symbolName string) ([]RelationshipResult, error) {
	return queryRelationships(db,
		`r.to_symbol_id IN (SELECT id FROM symbols WHERE name = ? OR qualified_name = ?)
		 AND r.reference_kind = 'calls'`,
		symbolName, symbolName)
}

// Calls returns references where from_symbol matches the given symbol name (outgoing calls).
func Calls(db *sql.DB, symbolName string) ([]RelationshipResult, error) {
	return queryRelationships(db,
		`r.from_symbol_id IN (SELECT id FROM symbols WHERE name = ? OR qualified_name = ?)
		 AND r.reference_kind = 'calls'`,
		symbolName, symbolName)
}

// Implementations returns references where reference_kind is 'implements' for the given interface.
func Implementations(db *sql.DB, interfaceName string) ([]RelationshipResult, error) {
	return queryRelationships(db,
		`r.to_symbol_id IN (SELECT id FROM symbols WHERE name = ? OR qualified_name = ?)
		 AND r.reference_kind = 'implements'`,
		interfaceName, interfaceName)
}

// Imports returns references where reference_kind is 'imports' for the given package.
func Imports(db *sql.DB, packageName string) ([]RelationshipResult, error) {
	return queryRelationships(db,
		`(r.raw_target_text = ? OR r.raw_target_text LIKE ?)
		 AND r.reference_kind = 'imports'`,
		packageName, "%/"+packageName)
}

// TestsFor returns references where reference_kind is 'tests' for the given target.
func TestsFor(db *sql.DB, targetName string) ([]RelationshipResult, error) {
	return queryRelationships(db,
		`r.to_symbol_id IN (SELECT id FROM symbols WHERE name = ? OR qualified_name = ?)
		 AND r.reference_kind = 'tests'`,
		targetName, targetName)
}

// Touches returns artifact-based references matching the given artifact kind and name.
func Touches(db *sql.DB, artifactKind, name string) ([]RelationshipResult, error) {
	query := `SELECT r.id,
		COALESCE(sf.qualified_name, ''), ff.path,
		COALESCE(st.qualified_name, ''), COALESCE(tf.path, ''),
		r.reference_kind, r.confidence, r.line, r.column_start, r.column_end,
		r.raw_target_text, r.is_resolved
		FROM "references" r
		JOIN files ff ON r.from_file_id = ff.id
		LEFT JOIN symbols sf ON r.from_symbol_id = sf.id
		LEFT JOIN symbols st ON r.to_symbol_id = st.id
		LEFT JOIN files tf ON r.to_file_id = tf.id
		WHERE r.from_file_id IN (
			SELECT a.file_id FROM artifacts a WHERE a.artifact_kind = ? AND a.name LIKE ?
		)
		ORDER BY ff.path, r.line`

	rows, err := db.Query(query, artifactKind, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanRelationships(rows)
}

func queryRelationships(db *sql.DB, whereClause string, args ...interface{}) ([]RelationshipResult, error) {
	query := `SELECT r.id,
		COALESCE(sf.qualified_name, ''), ff.path,
		COALESCE(st.qualified_name, ''), COALESCE(tf.path, ''),
		r.reference_kind, r.confidence, r.line, r.column_start, r.column_end,
		r.raw_target_text, r.is_resolved
		FROM "references" r
		JOIN files ff ON r.from_file_id = ff.id
		LEFT JOIN symbols sf ON r.from_symbol_id = sf.id
		LEFT JOIN symbols st ON r.to_symbol_id = st.id
		LEFT JOIN files tf ON r.to_file_id = tf.id
		WHERE ` + whereClause + `
		ORDER BY r.confidence, ff.path, r.line`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanRelationships(rows)
}

func scanRelationships(rows *sql.Rows) ([]RelationshipResult, error) {
	var results []RelationshipResult
	for rows.Next() {
		var r RelationshipResult
		var line, colStart, colEnd sql.NullInt64
		var rawTarget sql.NullString
		var isResolved int

		if err := rows.Scan(&r.ReferenceID, &r.FromSymbol, &r.FromFile,
			&r.ToSymbol, &r.ToFile, &r.ReferenceKind, &r.Confidence,
			&line, &colStart, &colEnd, &rawTarget, &isResolved); err != nil {
			return nil, err
		}
		if line.Valid {
			v := line.Int64
			r.Line = &v
		}
		if colStart.Valid {
			v := colStart.Int64
			r.ColumnStart = &v
		}
		if colEnd.Valid {
			v := colEnd.Int64
			r.ColumnEnd = &v
		}
		if rawTarget.Valid {
			r.RawTargetText = rawTarget.String
		}
		r.IsResolved = isResolved == 1
		results = append(results, r)
	}
	return results, rows.Err()
}
