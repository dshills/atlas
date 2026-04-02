package store

import (
	"database/sql"
	"time"
)

// RunRow represents a row in the index_runs table.
type RunRow struct {
	ID                int64
	StartedAt         string
	FinishedAt        sql.NullString
	Status            string
	Mode              string
	FilesScanned      int
	FilesChanged      int
	FilesReparsed     int
	SymbolsWritten    int
	ReferencesWritten int
	SummariesWritten  int
	ArtifactsWritten  int
	ErrorCount        int
	WarningCount      int
	GitCommit         sql.NullString
	Notes             sql.NullString
}

// InsertRun creates a new index run and returns its ID.
func (s *Store) InsertRun(mode string, gitCommit string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var gc sql.NullString
	if gitCommit != "" {
		gc = sql.NullString{String: gitCommit, Valid: true}
	}

	res, err := s.DB.Exec(`INSERT INTO index_runs (started_at, status, mode, git_commit) VALUES (?, 'running', ?, ?)`,
		now, mode, gc)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishRun updates a run with final stats and status.
func (s *Store) FinishRun(run *RunRow) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`UPDATE index_runs SET finished_at=?, status=?, files_scanned=?, files_changed=?, files_reparsed=?, symbols_written=?, references_written=?, summaries_written=?, artifacts_written=?, error_count=?, warning_count=?, notes=? WHERE id=?`,
		now, run.Status, run.FilesScanned, run.FilesChanged, run.FilesReparsed, run.SymbolsWritten, run.ReferencesWritten, run.SummariesWritten, run.ArtifactsWritten, run.ErrorCount, run.WarningCount, run.Notes, run.ID)
	return err
}

// LatestRun returns the most recent index run.
func (s *Store) LatestRun() (*RunRow, error) {
	r := &RunRow{}
	err := s.DB.QueryRow(`SELECT id, started_at, finished_at, status, mode, files_scanned, files_changed, files_reparsed, symbols_written, references_written, summaries_written, artifacts_written, error_count, warning_count, git_commit, notes FROM index_runs ORDER BY id DESC LIMIT 1`).
		Scan(&r.ID, &r.StartedAt, &r.FinishedAt, &r.Status, &r.Mode, &r.FilesScanned, &r.FilesChanged, &r.FilesReparsed, &r.SymbolsWritten, &r.ReferencesWritten, &r.SummariesWritten, &r.ArtifactsWritten, &r.ErrorCount, &r.WarningCount, &r.GitCommit, &r.Notes)
	if err != nil {
		return nil, err
	}
	return r, nil
}
