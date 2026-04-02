// Package validate implements index integrity validation for Atlas.
package validate

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dshills/atlas/internal/fswalk"
)

// CheckResult represents the outcome of a single validation check.
type CheckResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"` // "pass" or "fail"
	Details    string `json:"details,omitempty"`
	Violations int    `json:"violations"`
}

// Report contains the results of all validation checks.
type Report struct {
	Checks []CheckResult `json:"checks"`
	Valid  bool          `json:"valid"`
}

// Options controls validation behavior.
type Options struct {
	Strict   bool
	RepoRoot string
	Include  []string
	Exclude  []string
	MaxSize  int64
}

// Run executes all validation checks.
func Run(database *sql.DB, opts Options) *Report {
	r := &Report{Valid: true}

	r.add(checkForeignKeys(database))
	r.add(checkDuplicateStableIDs(database))
	r.add(checkOrphanedSymbols(database))
	r.add(checkOrphanedPackageFiles(database))
	r.add(checkStaleSummaries(database))
	r.add(checkFilesOnDisk(database, opts.RepoRoot))
	r.add(checkDenormalizedPackageName(database))
	r.add(checkDenormalizedModuleName(database))
	r.add(checkSummaryTextLength(database))
	r.add(checkSummaryArrayEntryLength(database))
	r.add(checkArtifactDataJSON(database))

	if opts.Strict {
		r.add(checkUnindexedFiles(database, opts))
	}

	return r
}

func (r *Report) add(c CheckResult) {
	r.Checks = append(r.Checks, c)
	if c.Status == "fail" {
		r.Valid = false
	}
}

func checkForeignKeys(database *sql.DB) CheckResult {
	rows, err := database.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return CheckResult{Name: "foreign_keys", Status: "fail", Details: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		count++
	}
	if count > 0 {
		return CheckResult{Name: "foreign_keys", Status: "fail", Details: fmt.Sprintf("%d FK violations", count), Violations: count}
	}
	return CheckResult{Name: "foreign_keys", Status: "pass"}
}

func checkDuplicateStableIDs(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM (SELECT stable_id FROM symbols GROUP BY stable_id HAVING COUNT(*) > 1)`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "duplicate_stable_ids", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "duplicate_stable_ids", Status: "fail", Details: fmt.Sprintf("%d duplicate stable_ids", count), Violations: count}
	}
	return CheckResult{Name: "duplicate_stable_ids", Status: "pass"}
}

func checkOrphanedSymbols(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM symbols s WHERE NOT EXISTS (SELECT 1 FROM files f WHERE f.id = s.file_id)`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "orphaned_symbols", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "orphaned_symbols", Status: "fail", Details: fmt.Sprintf("%d symbols with invalid file_id", count), Violations: count}
	}
	return CheckResult{Name: "orphaned_symbols", Status: "pass"}
}

func checkOrphanedPackageFiles(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM package_files pf WHERE NOT EXISTS (SELECT 1 FROM packages p WHERE p.id = pf.package_id) OR NOT EXISTS (SELECT 1 FROM files f WHERE f.id = pf.file_id)`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "orphaned_package_files", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "orphaned_package_files", Status: "fail", Details: fmt.Sprintf("%d orphaned entries", count), Violations: count}
	}
	return CheckResult{Name: "orphaned_package_files", Status: "pass"}
}

func checkStaleSummaries(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM file_summaries fs JOIN files f ON fs.file_id = f.id WHERE fs.generated_from_hash != f.content_hash`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "stale_summaries", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "stale_summaries", Status: "fail", Details: fmt.Sprintf("%d stale summaries", count), Violations: count}
	}
	return CheckResult{Name: "stale_summaries", Status: "pass"}
}

func checkFilesOnDisk(database *sql.DB, repoRoot string) CheckResult {
	rows, err := database.Query(`SELECT path FROM files`)
	if err != nil {
		return CheckResult{Name: "files_on_disk", Status: "fail", Details: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	missing := 0
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, path)); os.IsNotExist(err) {
			missing++
		}
	}
	if missing > 0 {
		return CheckResult{Name: "files_on_disk", Status: "fail", Details: fmt.Sprintf("%d files missing", missing), Violations: missing}
	}
	return CheckResult{Name: "files_on_disk", Status: "pass"}
}

func checkDenormalizedPackageName(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM files f JOIN package_files pf ON f.id = pf.file_id JOIN packages p ON pf.package_id = p.id WHERE f.package_name IS NOT NULL AND f.package_name != p.name`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "denorm_package_name", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "denorm_package_name", Status: "fail", Details: fmt.Sprintf("%d mismatches", count), Violations: count}
	}
	return CheckResult{Name: "denorm_package_name", Status: "pass"}
}

func checkDenormalizedModuleName(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM files f JOIN package_files pf ON f.id = pf.file_id JOIN packages p ON pf.package_id = p.id WHERE f.module_name IS NOT NULL AND p.import_path IS NOT NULL AND f.module_name != p.import_path`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "denorm_module_name", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "denorm_module_name", Status: "fail", Details: fmt.Sprintf("%d mismatches", count), Violations: count}
	}
	return CheckResult{Name: "denorm_module_name", Status: "pass"}
}

func checkSummaryTextLength(database *sql.DB) CheckResult {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM (
		SELECT summary_text FROM file_summaries WHERE LENGTH(summary_text) > 500
		UNION ALL SELECT summary_text FROM package_summaries WHERE LENGTH(summary_text) > 500
		UNION ALL SELECT summary_text FROM symbol_summaries WHERE LENGTH(summary_text) > 500
	)`).Scan(&count)
	if err != nil {
		return CheckResult{Name: "summary_text_length", Status: "fail", Details: err.Error()}
	}
	if count > 0 {
		return CheckResult{Name: "summary_text_length", Status: "fail", Details: fmt.Sprintf("%d summaries exceed 500 chars", count), Violations: count}
	}
	return CheckResult{Name: "summary_text_length", Status: "pass"}
}

func checkSummaryArrayEntryLength(database *sql.DB) CheckResult {
	violations := 0
	jsonCols := []struct {
		table, col string
	}{
		{"file_summaries", "responsibilities_json"},
		{"file_summaries", "key_symbols_json"},
		{"file_summaries", "dependencies_json"},
		{"file_summaries", "public_api_json"},
		{"package_summaries", "major_responsibilities_json"},
		{"package_summaries", "exported_surface_json"},
		{"symbol_summaries", "intent_json"},
		{"symbol_summaries", "related_symbols_json"},
	}

	for _, jc := range jsonCols {
		rows, err := database.Query(fmt.Sprintf(`SELECT %s FROM %q WHERE %s IS NOT NULL`, jc.col, jc.table, jc.col))
		if err != nil {
			continue
		}
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				continue
			}
			var entries []string
			if err := json.Unmarshal([]byte(data), &entries); err != nil {
				continue
			}
			for _, e := range entries {
				if len(e) > 100 {
					violations++
				}
			}
		}
		_ = rows.Close()
	}

	if violations > 0 {
		return CheckResult{Name: "summary_array_entry_length", Status: "fail", Details: fmt.Sprintf("%d entries exceed 100 chars", violations), Violations: violations}
	}
	return CheckResult{Name: "summary_array_entry_length", Status: "pass"}
}

func checkArtifactDataJSON(database *sql.DB) CheckResult {
	requiredKeys := map[string][]string{
		"route":            {"method", "path"},
		"config_key":       {"key"},
		"migration":        {"table"},
		"sql_query":        {"table"},
		"background_job":   {"type"},
		"queue_consumer":   {"type"},
		"external_service": {"url"},
		"cli_command":      {"name"},
		"env_var":          {"key"},
		"feature_flag":     {"key"},
	}

	rows, err := database.Query(`SELECT artifact_kind, data_json FROM artifacts`)
	if err != nil {
		return CheckResult{Name: "artifact_data_json", Status: "fail", Details: err.Error()}
	}
	defer func() { _ = rows.Close() }()

	violations := 0
	for rows.Next() {
		var kind, dataJSON string
		if err := rows.Scan(&kind, &dataJSON); err != nil {
			continue
		}
		required, ok := requiredKeys[kind]
		if !ok {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			violations++
			continue
		}
		for _, key := range required {
			if _, exists := data[key]; !exists {
				violations++
			}
		}
	}

	if violations > 0 {
		return CheckResult{Name: "artifact_data_json", Status: "fail", Details: fmt.Sprintf("%d violations", violations), Violations: violations}
	}
	return CheckResult{Name: "artifact_data_json", Status: "pass"}
}

func checkUnindexedFiles(database *sql.DB, opts Options) CheckResult {
	candidates, err := fswalk.Walk(opts.RepoRoot, opts.Include, opts.Exclude, opts.MaxSize)
	if err != nil {
		return CheckResult{Name: "unindexed_files", Status: "fail", Details: fmt.Sprintf("walk error: %v", err)}
	}

	indexed := make(map[string]bool)
	rows, err := database.Query(`SELECT path FROM files`)
	if err != nil {
		return CheckResult{Name: "unindexed_files", Status: "fail", Details: err.Error()}
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		indexed[path] = true
	}

	missing := 0
	for _, c := range candidates {
		if !indexed[c.Path] {
			missing++
		}
	}

	if missing > 0 {
		return CheckResult{Name: "unindexed_files", Status: "fail", Details: fmt.Sprintf("%d files not indexed", missing), Violations: missing}
	}
	return CheckResult{Name: "unindexed_files", Status: "pass"}
}
