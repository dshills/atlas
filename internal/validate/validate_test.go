package validate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/atlas/internal/db"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestValidateCleanIndex(t *testing.T) {
	dir := t.TempDir()
	database := setupTestDB(t, dir)

	// Insert a file that exists on disk
	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := database.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, is_generated, parse_status, created_at, updated_at)
		VALUES (?, 'go', 'abc', 12, 0, 'ok', '2024-01-01', '2024-01-01')`, "main.go")
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{RepoRoot: dir}
	report := Run(database, opts)

	if !report.Valid {
		for _, c := range report.Checks {
			if c.Status == "fail" {
				t.Errorf("check %s failed: %s", c.Name, c.Details)
			}
		}
	}
}

func TestValidateDetectsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	database := setupTestDB(t, dir)

	// Insert file that does NOT exist on disk
	_, err := database.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, is_generated, parse_status, created_at, updated_at)
		VALUES ('nonexistent.go', 'go', 'abc', 12, 0, 'ok', '2024-01-01', '2024-01-01')`)
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{RepoRoot: dir}
	report := Run(database, opts)

	if report.Valid {
		t.Error("expected invalid report with missing file")
	}

	found := false
	for _, c := range report.Checks {
		if c.Name == "files_on_disk" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected files_on_disk check to fail")
	}
}

func TestValidateStaleSummaries(t *testing.T) {
	dir := t.TempDir()
	database := setupTestDB(t, dir)

	// Create file on disk
	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := database.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, is_generated, parse_status, created_at, updated_at)
		VALUES ('main.go', 'go', 'newhash', 12, 0, 'ok', '2024-01-01', '2024-01-01')`)
	if err != nil {
		t.Fatal(err)
	}
	fileID, _ := res.LastInsertId()

	// Insert a stale summary
	_, err = database.Exec(`INSERT INTO file_summaries (file_id, summary_text, generated_from_hash, generator_version, created_at, updated_at)
		VALUES (?, 'test summary', 'oldhash', '0.1.0', '2024-01-01', '2024-01-01')`, fileID)
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{RepoRoot: dir}
	report := Run(database, opts)

	found := false
	for _, c := range report.Checks {
		if c.Name == "stale_summaries" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected stale_summaries check to fail")
	}
}

func TestValidateStrictUnindexedFiles(t *testing.T) {
	dir := t.TempDir()
	database := setupTestDB(t, dir)

	// Create a Go file but don't index it
	if err := os.WriteFile(filepath.Join(dir, "unindexed.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Strict:   true,
		RepoRoot: dir,
		MaxSize:  10 * 1024 * 1024,
	}
	report := Run(database, opts)

	found := false
	for _, c := range report.Checks {
		if c.Name == "unindexed_files" && c.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected unindexed_files check to fail in strict mode")
	}
}
