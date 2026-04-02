package query

import (
	"database/sql"
	"os"
)

// RepoStats holds repository statistics.
type RepoStats struct {
	FilesByLanguage  map[string]int     `json:"filesByLanguage"`
	PackageCount     int                `json:"packageCount"`
	SymbolsByKind    map[string]int     `json:"symbolsByKind"`
	ReferencesByKind map[string]int     `json:"referencesByKind"`
	ArtifactsByKind  map[string]int     `json:"artifactsByKind"`
	StaleSummaries   StaleSummaryCounts `json:"staleSummaries"`
	LastRun          *LastRunInfo       `json:"lastRun,omitempty"`
	DBFileSizeBytes  int64              `json:"dbFileSizeBytes"`
}

// StaleSummaryCounts tracks stale counts by summary type.
type StaleSummaryCounts struct {
	File    int `json:"file"`
	Package int `json:"package"`
	Symbol  int `json:"symbol"`
}

// LastRunInfo holds information about the most recent index run.
type LastRunInfo struct {
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Status     string `json:"status"`
	Mode       string `json:"mode"`
}

// GetStats gathers comprehensive repository statistics.
func GetStats(db *sql.DB, dbPath string) (*RepoStats, error) {
	stats := &RepoStats{
		FilesByLanguage:  make(map[string]int),
		SymbolsByKind:    make(map[string]int),
		ReferencesByKind: make(map[string]int),
		ArtifactsByKind:  make(map[string]int),
	}

	// Files by language
	rows, err := db.Query(`SELECT language, COUNT(*) FROM files GROUP BY language`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var lang string
		var count int
		if err := rows.Scan(&lang, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats.FilesByLanguage[lang] = count
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Package count
	if err := db.QueryRow(`SELECT COUNT(*) FROM packages`).Scan(&stats.PackageCount); err != nil {
		return nil, err
	}

	// Symbols by kind
	rows, err = db.Query(`SELECT symbol_kind, COUNT(*) FROM symbols GROUP BY symbol_kind`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats.SymbolsByKind[kind] = count
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// References by kind
	rows, err = db.Query(`SELECT reference_kind, COUNT(*) FROM "references" GROUP BY reference_kind`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats.ReferencesByKind[kind] = count
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Artifacts by kind
	rows, err = db.Query(`SELECT artifact_kind, COUNT(*) FROM artifacts GROUP BY artifact_kind`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats.ArtifactsByKind[kind] = count
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Stale summary counts
	_ = db.QueryRow(`SELECT COUNT(*) FROM file_summaries fs
		JOIN files f ON fs.file_id = f.id
		WHERE fs.generated_from_hash != f.content_hash`).Scan(&stats.StaleSummaries.File)

	_ = db.QueryRow(`SELECT COUNT(*) FROM package_summaries ps
		WHERE EXISTS (
			SELECT 1 FROM package_files pf
			JOIN files f ON pf.file_id = f.id
			WHERE pf.package_id = ps.package_id
			AND f.content_hash != ps.generated_from_hash
		)`).Scan(&stats.StaleSummaries.Package)

	_ = db.QueryRow(`SELECT COUNT(*) FROM symbol_summaries ss
		JOIN symbols s ON ss.symbol_id = s.id
		JOIN files f ON s.file_id = f.id
		WHERE ss.generated_from_hash != f.content_hash`).Scan(&stats.StaleSummaries.Symbol)

	// Last run
	var lr LastRunInfo
	var finishedAt sql.NullString
	err = db.QueryRow(`SELECT started_at, finished_at, status, mode
		FROM index_runs ORDER BY id DESC LIMIT 1`).
		Scan(&lr.StartedAt, &finishedAt, &lr.Status, &lr.Mode)
	if err == nil {
		if finishedAt.Valid {
			lr.FinishedAt = finishedAt.String
		}
		stats.LastRun = &lr
	}

	// DB file size
	if dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			stats.DBFileSizeBytes = info.Size()
		}
	}

	return stats, nil
}
