package query

import (
	"database/sql"
)

// ArtifactResult represents an artifact query result.
type ArtifactResult struct {
	ID           int64  `json:"id"`
	ArtifactKind string `json:"artifactKind"`
	Name         string `json:"name"`
	FilePath     string `json:"filePath"`
	SymbolName   string `json:"symbolName,omitempty"`
	DataJSON     string `json:"data"`
	Confidence   string `json:"confidence"`
}

// FindArtifacts searches for artifacts by kind and name pattern.
func FindArtifacts(db *sql.DB, kind, namePattern string) ([]ArtifactResult, error) {
	query := `SELECT a.id, a.artifact_kind, a.name, f.path,
		COALESCE(s.qualified_name, ''), a.data_json, a.confidence
		FROM artifacts a
		JOIN files f ON a.file_id = f.id
		LEFT JOIN symbols s ON a.symbol_id = s.id
		WHERE a.artifact_kind = ? AND a.name LIKE ?
		ORDER BY a.name, f.path`

	rows, err := db.Query(query, kind, "%"+namePattern+"%")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ArtifactResult
	for rows.Next() {
		var r ArtifactResult
		if err := rows.Scan(&r.ID, &r.ArtifactKind, &r.Name, &r.FilePath, &r.SymbolName, &r.DataJSON, &r.Confidence); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ListArtifactsByKind returns all artifacts of the given kind.
func ListArtifactsByKind(db *sql.DB, kind string) ([]ArtifactResult, error) {
	query := `SELECT a.id, a.artifact_kind, a.name, f.path,
		COALESCE(s.qualified_name, ''), a.data_json, a.confidence
		FROM artifacts a
		JOIN files f ON a.file_id = f.id
		LEFT JOIN symbols s ON a.symbol_id = s.id
		WHERE a.artifact_kind = ?
		ORDER BY a.name, f.path`

	rows, err := db.Query(query, kind)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ArtifactResult
	for rows.Next() {
		var r ArtifactResult
		if err := rows.Scan(&r.ID, &r.ArtifactKind, &r.Name, &r.FilePath, &r.SymbolName, &r.DataJSON, &r.Confidence); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
