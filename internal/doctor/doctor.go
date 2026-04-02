// Package doctor implements health checks for an Atlas repository.
package doctor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/db"
	"github.com/dshills/atlas/internal/model"
)

// CheckResult represents the outcome of a single health check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass" or "fail"
	Details string `json:"details,omitempty"`
}

// Report contains the results of all doctor checks.
type Report struct {
	Checks  []CheckResult `json:"checks"`
	Healthy bool          `json:"healthy"`
}

// Run executes all doctor checks against the given repo root.
func Run(repoRoot string) *Report {
	r := &Report{Healthy: true}
	storageDir := filepath.Join(repoRoot, config.DefaultStorageDir)

	// Check 1: .atlas/ directory exists and is writable
	r.add(checkStorageDir(storageDir))

	// Check 2: DB opens and responds to a test query
	dbPath := filepath.Join(storageDir, config.DefaultDBFile)
	database, dbErr := checkDBOpen(dbPath)
	r.add(dbCheckResult(dbErr))

	if database != nil {
		defer func() { _ = database.Close() }()

		// Check 3: schema version compatible
		r.add(checkSchemaVersion(database))

		// Check 5: error/warning counts from latest run
		r.add(checkLatestRun(database))

		// Check 6: stale summaries
		r.add(checkStaleSummaries(database))

		// Check 7: files in DB but missing on disk
		r.add(checkMissingFiles(database, repoRoot))

		// Check 8: SQLite integrity
		r.add(checkIntegrity(database))
	}

	// Check 4: manifest.json
	r.add(checkManifest(storageDir, repoRoot))

	return r
}

func (r *Report) add(c CheckResult) {
	r.Checks = append(r.Checks, c)
	if c.Status == "fail" {
		r.Healthy = false
	}
}

func checkStorageDir(storageDir string) CheckResult {
	info, err := os.Stat(storageDir)
	if err != nil {
		return CheckResult{Name: "storage_directory", Status: "fail", Details: fmt.Sprintf("not found: %v", err)}
	}
	if !info.IsDir() {
		return CheckResult{Name: "storage_directory", Status: "fail", Details: "exists but is not a directory"}
	}

	// Check writable by creating a temp file
	tmp := filepath.Join(storageDir, ".doctor_test")
	if err := os.WriteFile(tmp, []byte("test"), 0o644); err != nil {
		return CheckResult{Name: "storage_directory", Status: "fail", Details: fmt.Sprintf("not writable: %v", err)}
	}
	_ = os.Remove(tmp)

	return CheckResult{Name: "storage_directory", Status: "pass", Details: storageDir}
}

func checkDBOpen(dbPath string) (*sql.DB, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file not found: %s", dbPath)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open: %v", err)
	}

	var result int
	if err := database.QueryRow(`SELECT 1`).Scan(&result); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("test query failed: %v", err)
	}
	return database, nil
}

func dbCheckResult(err error) CheckResult {
	if err != nil {
		return CheckResult{Name: "database", Status: "fail", Details: err.Error()}
	}
	return CheckResult{Name: "database", Status: "pass", Details: "opens and responds"}
}

func checkSchemaVersion(database *sql.DB) CheckResult {
	if err := db.CheckSchemaVersion(database); err != nil {
		return CheckResult{Name: "schema_version", Status: "fail", Details: err.Error()}
	}
	return CheckResult{Name: "schema_version", Status: "pass", Details: fmt.Sprintf("version %d", db.SchemaVersion)}
}

func checkManifest(storageDir, repoRoot string) CheckResult {
	manifestPath := filepath.Join(storageDir, config.DefaultManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return CheckResult{Name: "manifest", Status: "fail", Details: fmt.Sprintf("cannot read: %v", err)}
	}

	var m model.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return CheckResult{Name: "manifest", Status: "fail", Details: fmt.Sprintf("invalid JSON: %v", err)}
	}

	if m.RepoRoot != repoRoot {
		return CheckResult{Name: "manifest", Status: "fail", Details: fmt.Sprintf("repo root mismatch: manifest=%s detected=%s", m.RepoRoot, repoRoot)}
	}

	return CheckResult{Name: "manifest", Status: "pass", Details: fmt.Sprintf("repo_root=%s schema_version=%d", m.RepoRoot, m.SchemaVersion)}
}

func checkLatestRun(database *sql.DB) CheckResult {
	var status string
	var errorCount, warningCount int
	err := database.QueryRow(`SELECT status, error_count, warning_count FROM index_runs ORDER BY id DESC LIMIT 1`).
		Scan(&status, &errorCount, &warningCount)
	if err == sql.ErrNoRows {
		return CheckResult{Name: "latest_run", Status: "pass", Details: "no runs yet"}
	}
	if err != nil {
		return CheckResult{Name: "latest_run", Status: "fail", Details: fmt.Sprintf("query error: %v", err)}
	}

	details := fmt.Sprintf("status=%s errors=%d warnings=%d", status, errorCount, warningCount)
	if errorCount > 0 {
		return CheckResult{Name: "latest_run", Status: "fail", Details: details}
	}
	return CheckResult{Name: "latest_run", Status: "pass", Details: details}
}

func checkStaleSummaries(database *sql.DB) CheckResult {
	var staleFiles, stalePkgs, staleSyms int

	if err := database.QueryRow(`SELECT COUNT(*) FROM file_summaries fs JOIN files f ON fs.file_id = f.id WHERE fs.generated_from_hash != f.content_hash`).Scan(&staleFiles); err != nil {
		return CheckResult{Name: "stale_summaries", Status: "fail", Details: fmt.Sprintf("query error: %v", err)}
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM package_summaries ps WHERE EXISTS (SELECT 1 FROM package_files pf JOIN files f ON pf.file_id = f.id WHERE pf.package_id = ps.package_id AND f.content_hash != ps.generated_from_hash)`).Scan(&stalePkgs); err != nil {
		return CheckResult{Name: "stale_summaries", Status: "fail", Details: fmt.Sprintf("query error: %v", err)}
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM symbol_summaries ss JOIN symbols s ON ss.symbol_id = s.id JOIN files f ON s.file_id = f.id WHERE ss.generated_from_hash != f.content_hash`).Scan(&staleSyms); err != nil {
		return CheckResult{Name: "stale_summaries", Status: "fail", Details: fmt.Sprintf("query error: %v", err)}
	}

	total := staleFiles + stalePkgs + staleSyms
	details := fmt.Sprintf("stale: files=%d packages=%d symbols=%d", staleFiles, stalePkgs, staleSyms)
	if total > 0 {
		return CheckResult{Name: "stale_summaries", Status: "fail", Details: details}
	}
	return CheckResult{Name: "stale_summaries", Status: "pass", Details: details}
}

func checkMissingFiles(database *sql.DB, repoRoot string) CheckResult {
	rows, err := database.Query(`SELECT path FROM files`)
	if err != nil {
		return CheckResult{Name: "missing_files", Status: "fail", Details: fmt.Sprintf("query error: %v", err)}
	}
	defer func() { _ = rows.Close() }()

	missing := 0
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		absPath := filepath.Join(repoRoot, path)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			missing++
		}
	}

	details := fmt.Sprintf("%d files in DB missing on disk", missing)
	if missing > 0 {
		return CheckResult{Name: "missing_files", Status: "fail", Details: details}
	}
	return CheckResult{Name: "missing_files", Status: "pass", Details: details}
}

func checkIntegrity(database *sql.DB) CheckResult {
	var result string
	if err := database.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return CheckResult{Name: "integrity", Status: "fail", Details: fmt.Sprintf("check failed: %v", err)}
	}
	if result != "ok" {
		return CheckResult{Name: "integrity", Status: "fail", Details: result}
	}
	return CheckResult{Name: "integrity", Status: "pass", Details: "ok"}
}
