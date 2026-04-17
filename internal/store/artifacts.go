package store

import (
	"database/sql"
	"time"

	"github.com/dshills/atlas/internal/extractor"
)

// UpsertArtifacts deletes existing artifacts for a file and inserts new ones.
func (s *Store) UpsertArtifacts(tx Execer, fileID int64, artifacts []extractor.ArtifactRecord) (int, error) {
	if _, err := tx.Exec(`DELETE FROM artifacts WHERE file_id = ?`, fileID); err != nil {
		return 0, err
	}

	if len(artifacts) == 0 {
		return 0, nil
	}

	stmt, err := tx.Prepare(`INSERT INTO artifacts (artifact_kind, name, file_id, symbol_id, data_json, confidence, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0

	for _, a := range artifacts {
		var symID sql.NullInt64
		// Symbol association will be resolved later if needed

		if _, err := stmt.Exec(a.ArtifactKind, a.Name, fileID, symID, a.DataJSON, a.Confidence, now, now); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
