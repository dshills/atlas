package query

import (
	"database/sql"
	"sort"
	"strings"
)

// SymbolResult represents a symbol query result with ranking metadata.
type SymbolResult struct {
	ID            int64  `json:"id"`
	FileID        int64  `json:"fileId"`
	PackageID     *int64 `json:"packageId,omitempty"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualifiedName"`
	SymbolKind    string `json:"symbolKind"`
	Visibility    string `json:"visibility"`
	Signature     string `json:"signature,omitempty"`
	DocComment    string `json:"docComment,omitempty"`
	StartLine     *int64 `json:"startLine,omitempty"`
	EndLine       *int64 `json:"endLine,omitempty"`
	StableID      string `json:"stableId"`
	FilePath      string `json:"filePath"`
	FileUpdatedAt string `json:"fileUpdatedAt"`

	// MatchType indicates how this result was matched.
	MatchType string `json:"matchType"`
}

// SymbolOptions controls symbol query behavior.
type SymbolOptions struct {
	Fuzzy      bool
	Kind       string
	Package    string
	File       string
	Language   string
	Visibility string
}

// FindSymbol resolves a symbol using the resolution order from Section 17.1:
// 1. exact qualified_name match
// 2. exact stable_id match
// 3. exact name match
// 4. case-insensitive substring match (when fuzzy=true)
// Results are ranked per Section 17.2.
func FindSymbol(db *sql.DB, name string, opts SymbolOptions) ([]SymbolResult, error) {
	var results []SymbolResult

	// Step 1: exact qualified_name match
	rows, err := querySymbols(db, `s.qualified_name = ?`, name, opts)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].MatchType = "exact_qualified"
	}
	results = append(results, rows...)

	// Step 2: exact stable_id match
	rows, err = querySymbols(db, `s.stable_id = ?`, name, opts)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].MatchType = "exact_stable_id"
	}
	results = append(results, dedup(results, rows)...)

	// Step 3: exact name match
	rows, err = querySymbols(db, `s.name = ?`, name, opts)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].MatchType = "exact_name"
	}
	results = append(results, dedup(results, rows)...)

	// Step 4: fuzzy match (case-insensitive substring)
	if opts.Fuzzy {
		pattern := "%" + name + "%"
		rows, err = querySymbols(db, `LOWER(s.name) LIKE LOWER(?) OR LOWER(s.qualified_name) LIKE LOWER(?)`, pattern, opts, pattern)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			rows[i].MatchType = "fuzzy"
		}
		results = append(results, dedup(results, rows)...)
	}

	rankSymbols(results)
	return results, nil
}

func querySymbols(db *sql.DB, whereClause string, nameArg string, opts SymbolOptions, extraArgs ...interface{}) ([]SymbolResult, error) {
	query := `SELECT s.id, s.file_id, s.package_id, s.name, s.qualified_name, s.symbol_kind,
		s.visibility, s.signature, s.doc_comment, s.start_line, s.end_line, s.stable_id,
		f.path, f.updated_at
		FROM symbols s
		JOIN files f ON s.file_id = f.id
		WHERE ` + whereClause

	args := []interface{}{nameArg}
	args = append(args, extraArgs...)

	if opts.Kind != "" {
		query += ` AND s.symbol_kind = ?`
		args = append(args, opts.Kind)
	}
	if opts.Visibility != "" {
		query += ` AND s.visibility = ?`
		args = append(args, opts.Visibility)
	}
	if opts.Package != "" {
		query += ` AND s.package_id IN (SELECT id FROM packages WHERE name = ? OR import_path = ?)`
		args = append(args, opts.Package, opts.Package)
	}
	if opts.File != "" {
		query += ` AND f.path LIKE ?`
		args = append(args, "%"+opts.File+"%")
	}
	if opts.Language != "" {
		query += ` AND f.language = ?`
		args = append(args, opts.Language)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []SymbolResult
	for rows.Next() {
		var r SymbolResult
		var pkgID sql.NullInt64
		var sig, doc sql.NullString
		var startLine, endLine sql.NullInt64

		if err := rows.Scan(&r.ID, &r.FileID, &pkgID, &r.Name, &r.QualifiedName, &r.SymbolKind,
			&r.Visibility, &sig, &doc, &startLine, &endLine, &r.StableID,
			&r.FilePath, &r.FileUpdatedAt); err != nil {
			return nil, err
		}
		if pkgID.Valid {
			v := pkgID.Int64
			r.PackageID = &v
		}
		if sig.Valid {
			r.Signature = sig.String
		}
		if doc.Valid {
			r.DocComment = doc.String
		}
		if startLine.Valid {
			v := startLine.Int64
			r.StartLine = &v
		}
		if endLine.Valid {
			v := endLine.Int64
			r.EndLine = &v
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// dedup returns only items from candidates that are not already in existing (by ID).
func dedup(existing, candidates []SymbolResult) []SymbolResult {
	seen := make(map[int64]bool, len(existing))
	for _, r := range existing {
		seen[r.ID] = true
	}
	var out []SymbolResult
	for _, r := range candidates {
		if !seen[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// rankSymbols sorts results per Section 17.2:
// 1. Exactness: exact_qualified > exact_stable_id > exact_name > fuzzy
// 2. Visibility: exported > unexported
// 3. Symbol kind: functions/types rank above fields/variables/constants
// 4. File modification recency
func rankSymbols(results []SymbolResult) {
	matchOrder := map[string]int{
		"exact_qualified": 0,
		"exact_stable_id": 1,
		"exact_name":      2,
		"fuzzy":           3,
	}
	kindOrder := map[string]int{
		"function":   0,
		"method":     0,
		"struct":     1,
		"interface":  1,
		"type":       1,
		"class":      1,
		"entrypoint": 1,
		"test":       2,
		"benchmark":  2,
		"package":    3,
		"module":     3,
		"const":      4,
		"var":        4,
		"field":      5,
		"enum":       3,
		"trait":      1,
		"protocol":   1,
	}

	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]

		// 1. Exactness
		ma, mb := matchOrder[a.MatchType], matchOrder[b.MatchType]
		if ma != mb {
			return ma < mb
		}

		// 2. Visibility
		va := visOrder(a.Visibility)
		vb := visOrder(b.Visibility)
		if va != vb {
			return va < vb
		}

		// 3. Symbol kind
		ka, kb := kindOrder[a.SymbolKind], kindOrder[b.SymbolKind]
		if ka != kb {
			return ka < kb
		}

		// 4. Recency (lexicographic comparison of RFC3339 timestamps, descending)
		return strings.Compare(a.FileUpdatedAt, b.FileUpdatedAt) > 0
	})
}

func visOrder(v string) int {
	if v == "exported" {
		return 0
	}
	return 1
}
