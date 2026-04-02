package store

import (
	"database/sql"
	"time"

	"github.com/dshills/atlas/internal/extractor"
)

// UpsertArtifacts deletes existing artifacts for a file and inserts new ones.
func (s *Store) UpsertArtifacts(fileID int64, artifacts []extractor.ArtifactRecord) (int, error) {
	if _, err := s.DB.Exec(`DELETE FROM artifacts WHERE file_id = ?`, fileID); err != nil {
		return 0, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0

	for _, a := range artifacts {
		var symID sql.NullInt64
		// Symbol association will be resolved later if needed

		_, err := s.DB.Exec(`INSERT INTO artifacts (artifact_kind, name, file_id, symbol_id, data_json, confidence, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ArtifactKind, a.Name, fileID, symID, a.DataJSON, a.Confidence, now, now)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
