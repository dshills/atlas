package query

import (
	"database/sql"
)

// StaleSummary represents a summary whose generated_from_hash differs from the current content_hash.
type StaleSummary struct {
	Kind         string `json:"kind"` // file, package, symbol
	EntityID     int64  `json:"entityId"`
	EntityName   string `json:"entityName"`
	StoredHash   string `json:"storedHash"`
	CurrentHash  string `json:"currentHash"`
	GeneratorVer string `json:"generatorVersion"`
}

// FindStaleSummaries returns all summaries where the generated_from_hash
// does not match the current file content_hash.
func FindStaleSummaries(db *sql.DB) ([]StaleSummary, error) {
	var results []StaleSummary

	// Stale file summaries
	rows, err := db.Query(`SELECT fs.file_id, f.path, fs.generated_from_hash, f.content_hash, fs.generator_version
		FROM file_summaries fs
		JOIN files f ON fs.file_id = f.id
		WHERE fs.generated_from_hash != f.content_hash`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s StaleSummary
		s.Kind = "file"
		if err := rows.Scan(&s.EntityID, &s.EntityName, &s.StoredHash, &s.CurrentHash, &s.GeneratorVer); err != nil {
			_ = rows.Close()
			return nil, err
		}
		results = append(results, s)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Stale package summaries: compare generated_from_hash against a composite
	// of all files in the package. A package summary is stale if any file changed.
	rows, err = db.Query(`SELECT ps.package_id, p.name, ps.generated_from_hash, ps.generator_version
		FROM package_summaries ps
		JOIN packages p ON ps.package_id = p.id
		WHERE EXISTS (
			SELECT 1 FROM package_files pf
			JOIN files f ON pf.file_id = f.id
			WHERE pf.package_id = ps.package_id
			AND f.content_hash != ps.generated_from_hash
		)`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s StaleSummary
		s.Kind = "package"
		if err := rows.Scan(&s.EntityID, &s.EntityName, &s.StoredHash, &s.GeneratorVer); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.CurrentHash = "(multiple files changed)"
		results = append(results, s)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Stale symbol summaries
	rows, err = db.Query(`SELECT ss.symbol_id, s.qualified_name, ss.generated_from_hash, f.content_hash, ss.generator_version
		FROM symbol_summaries ss
		JOIN symbols s ON ss.symbol_id = s.id
		JOIN files f ON s.file_id = f.id
		WHERE ss.generated_from_hash != f.content_hash`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s StaleSummary
		s.Kind = "symbol"
		if err := rows.Scan(&s.EntityID, &s.EntityName, &s.StoredHash, &s.CurrentHash, &s.GeneratorVer); err != nil {
			_ = rows.Close()
			return nil, err
		}
		results = append(results, s)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
