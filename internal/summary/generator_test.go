package summary

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/dshills/atlas/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "atlas-summary-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })
	_ = tmpFile.Close()

	database, err := db.Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func insertTestFile(t *testing.T, database *sql.DB, path, hash string) int64 {
	t.Helper()
	res, err := database.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, is_generated, parse_status, created_at, updated_at)
		VALUES (?, 'go', ?, 100, 0, 'ok', '2024-01-01', '2024-01-01')`, path, hash)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertTestPackage(t *testing.T, database *sql.DB, name, dirPath string) int64 {
	t.Helper()
	res, err := database.Exec(`INSERT INTO packages (name, directory_path, language, created_at, updated_at)
		VALUES (?, ?, 'go', '2024-01-01', '2024-01-01')`, name, dirPath)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertTestSymbol(t *testing.T, database *sql.DB, fileID, pkgID int64, name, qname, kind, vis string) int64 {
	t.Helper()
	stableID := "go:" + qname + ":" + kind
	res, err := database.Exec(`INSERT INTO symbols (file_id, package_id, name, qualified_name, symbol_kind, visibility, stable_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '2024-01-01', '2024-01-01')`, fileID, pkgID, name, qname, kind, vis, stableID)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestFileSummaryGeneration(t *testing.T) {
	database := setupTestDB(t)
	fileID := insertTestFile(t, database, "main.go", "abc123")
	pkgID := insertTestPackage(t, database, "main", ".")
	_, err := database.Exec(`INSERT INTO package_files (package_id, file_id) VALUES (?, ?)`, pkgID, fileID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`UPDATE files SET package_name = 'main' WHERE id = ?`, fileID)
	if err != nil {
		t.Fatal(err)
	}

	insertTestSymbol(t, database, fileID, pkgID, "main", "main.main", "entrypoint", "unexported")
	insertTestSymbol(t, database, fileID, pkgID, "Run", "main.Run", "function", "exported")

	gen := NewGenerator(database, "0.1.0")
	if err := gen.GenerateFileSummary(fileID); err != nil {
		t.Fatal(err)
	}

	// Verify summary was created
	var summaryText, genHash string
	err = database.QueryRow(`SELECT summary_text, generated_from_hash FROM file_summaries WHERE file_id = ?`, fileID).
		Scan(&summaryText, &genHash)
	if err != nil {
		t.Fatal(err)
	}
	if summaryText == "" {
		t.Error("expected non-empty summary text")
	}
	if len(summaryText) > MaxSummaryText {
		t.Errorf("summary text exceeds %d chars: %d", MaxSummaryText, len(summaryText))
	}
	if genHash != "abc123" {
		t.Errorf("expected generated_from_hash=abc123, got %s", genHash)
	}
}

func TestPackageSummaryGeneration(t *testing.T) {
	database := setupTestDB(t)
	fileID := insertTestFile(t, database, "pkg/util.go", "def456")
	pkgID := insertTestPackage(t, database, "pkg", "pkg")
	_, err := database.Exec(`INSERT INTO package_files (package_id, file_id) VALUES (?, ?)`, pkgID, fileID)
	if err != nil {
		t.Fatal(err)
	}

	insertTestSymbol(t, database, fileID, pkgID, "Helper", "pkg.Helper", "function", "exported")
	insertTestSymbol(t, database, fileID, pkgID, "internal", "pkg.internal", "function", "unexported")

	gen := NewGenerator(database, "0.1.0")
	if err := gen.GeneratePackageSummary(pkgID); err != nil {
		t.Fatal(err)
	}

	var summaryText string
	err = database.QueryRow(`SELECT summary_text FROM package_summaries WHERE package_id = ?`, pkgID).Scan(&summaryText)
	if err != nil {
		t.Fatal(err)
	}
	if summaryText == "" {
		t.Error("expected non-empty package summary")
	}
	if len(summaryText) > MaxSummaryText {
		t.Errorf("summary text exceeds %d chars", MaxSummaryText)
	}
}

func TestSymbolSummaryGeneration(t *testing.T) {
	database := setupTestDB(t)
	fileID := insertTestFile(t, database, "main.go", "ghi789")
	pkgID := insertTestPackage(t, database, "main", ".")
	symID := insertTestSymbol(t, database, fileID, pkgID, "Run", "main.Run", "function", "exported")

	gen := NewGenerator(database, "0.1.0")
	if err := gen.GenerateSymbolSummary(symID); err != nil {
		t.Fatal(err)
	}

	var summaryText, genHash string
	err := database.QueryRow(`SELECT summary_text, generated_from_hash FROM symbol_summaries WHERE symbol_id = ?`, symID).
		Scan(&summaryText, &genHash)
	if err != nil {
		t.Fatal(err)
	}
	if summaryText == "" {
		t.Error("expected non-empty symbol summary")
	}
	if genHash != "ghi789" {
		t.Errorf("expected hash=ghi789, got %s", genHash)
	}
}

func TestStaleSummaryDetection(t *testing.T) {
	database := setupTestDB(t)
	fileID := insertTestFile(t, database, "main.go", "hash1")

	gen := NewGenerator(database, "0.1.0")
	pkgID := insertTestPackage(t, database, "main", ".")
	insertTestSymbol(t, database, fileID, pkgID, "main", "main.main", "function", "unexported")
	if err := gen.GenerateFileSummary(fileID); err != nil {
		t.Fatal(err)
	}

	// Verify fresh
	var genHash string
	err := database.QueryRow(`SELECT generated_from_hash FROM file_summaries WHERE file_id = ?`, fileID).Scan(&genHash)
	if err != nil {
		t.Fatal(err)
	}
	if genHash != "hash1" {
		t.Fatalf("expected hash1, got %s", genHash)
	}

	// Simulate file change
	_, err = database.Exec(`UPDATE files SET content_hash = 'hash2' WHERE id = ?`, fileID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify stale (generated_from_hash != content_hash)
	var staleCount int
	err = database.QueryRow(`SELECT COUNT(*) FROM file_summaries fs JOIN files f ON fs.file_id = f.id WHERE fs.generated_from_hash != f.content_hash`).Scan(&staleCount)
	if err != nil {
		t.Fatal(err)
	}
	if staleCount != 1 {
		t.Errorf("expected 1 stale summary, got %d", staleCount)
	}
}

func TestTruncation(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	result := truncate(string(long), MaxSummaryText)
	if len(result) > MaxSummaryText {
		t.Errorf("truncated string exceeds max: %d", len(result))
	}

	entryResult := truncate(string(long), MaxArrayEntry)
	if len(entryResult) > MaxArrayEntry {
		t.Errorf("truncated entry exceeds max: %d", len(entryResult))
	}
}
