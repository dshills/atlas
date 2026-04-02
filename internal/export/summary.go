// Package export provides stable JSON export functions for Atlas data.
package export

import (
	"database/sql"
	"fmt"
)

// RepoSummary is the top-level export structure for atlas export summary.
type RepoSummary struct {
	RepoRoot    string           `json:"repo_root"`
	LastRun     *LastRunInfo     `json:"last_run"`
	Languages   []string         `json:"languages"`
	Packages    []PackageBrief   `json:"packages"`
	Entrypoints []string         `json:"entrypoints"`
	RouteCount  int              `json:"route_count"`
	Diagnostics DiagnosticCounts `json:"diagnostics"`
	StaleCounts StaleCounts      `json:"stale_counts"`
}

// LastRunInfo holds metadata about the most recent index run.
type LastRunInfo struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	Mode         string `json:"mode"`
	FilesScanned int    `json:"files_scanned"`
	FilesChanged int    `json:"files_changed"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
}

// PackageBrief holds summary info about a package.
type PackageBrief struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Language  string `json:"language"`
	FileCount int    `json:"file_count"`
}

// DiagnosticCounts holds error/warning counts.
type DiagnosticCounts struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// StaleCounts holds stale summary counts.
type StaleCounts struct {
	Files    int `json:"files"`
	Packages int `json:"packages"`
	Symbols  int `json:"symbols"`
}

// ExportSummary produces a RepoSummary from the database.
func ExportSummary(database *sql.DB, repoRoot string) (*RepoSummary, error) {
	s := &RepoSummary{
		RepoRoot:    repoRoot,
		Languages:   []string{},
		Packages:    []PackageBrief{},
		Entrypoints: []string{},
	}

	// Last run
	var lr LastRunInfo
	var finishedAt sql.NullString
	err := database.QueryRow(`SELECT id, status, mode, files_scanned, files_changed, started_at, COALESCE(finished_at,'')
		FROM index_runs ORDER BY id DESC LIMIT 1`).
		Scan(&lr.ID, &lr.Status, &lr.Mode, &lr.FilesScanned, &lr.FilesChanged, &lr.StartedAt, &finishedAt)
	if err == nil {
		lr.FinishedAt = finishedAt.String
		s.LastRun = &lr
	}

	// Languages
	rows, err := database.Query(`SELECT DISTINCT language FROM files`)
	if err != nil {
		return nil, fmt.Errorf("querying languages: %w", err)
	}
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.Languages = append(s.Languages, lang)
	}
	_ = rows.Close()

	// Packages with file count
	rows, err = database.Query(`SELECT p.name, p.directory_path, p.language, COUNT(pf.file_id)
		FROM packages p LEFT JOIN package_files pf ON p.id = pf.package_id GROUP BY p.id ORDER BY p.name`)
	if err != nil {
		return nil, fmt.Errorf("querying packages: %w", err)
	}
	for rows.Next() {
		var pb PackageBrief
		if err := rows.Scan(&pb.Name, &pb.Path, &pb.Language, &pb.FileCount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.Packages = append(s.Packages, pb)
	}
	_ = rows.Close()

	// Entrypoints
	rows, err = database.Query(`SELECT qualified_name FROM symbols WHERE symbol_kind = 'entrypoint'`)
	if err != nil {
		return nil, fmt.Errorf("querying entrypoints: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.Entrypoints = append(s.Entrypoints, name)
	}
	_ = rows.Close()

	// Route count
	_ = database.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE artifact_kind = 'route'`).Scan(&s.RouteCount)

	// Diagnostics from latest run
	if s.LastRun != nil {
		_ = database.QueryRow(`SELECT error_count, warning_count FROM index_runs WHERE id = ?`, s.LastRun.ID).
			Scan(&s.Diagnostics.Errors, &s.Diagnostics.Warnings)
	}

	// Stale counts
	_ = database.QueryRow(`SELECT COUNT(*) FROM file_summaries fs JOIN files f ON fs.file_id = f.id WHERE fs.generated_from_hash != f.content_hash`).Scan(&s.StaleCounts.Files)
	_ = database.QueryRow(`SELECT COUNT(*) FROM package_summaries ps WHERE EXISTS (SELECT 1 FROM package_files pf JOIN files f ON pf.file_id = f.id WHERE pf.package_id = ps.package_id AND f.content_hash != ps.generated_from_hash)`).Scan(&s.StaleCounts.Packages)
	_ = database.QueryRow(`SELECT COUNT(*) FROM symbol_summaries ss JOIN symbols s ON ss.symbol_id = s.id JOIN files f ON s.file_id = f.id WHERE ss.generated_from_hash != f.content_hash`).Scan(&s.StaleCounts.Symbols)

	return s, nil
}
