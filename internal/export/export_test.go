package export

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/dshills/atlas/internal/db"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "atlas-export-test-*.db")
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

func seedTestData(t *testing.T, database *sql.DB) {
	t.Helper()

	// Insert a run
	_, err := database.Exec(`INSERT INTO index_runs (started_at, status, mode, files_scanned, files_changed, error_count, warning_count)
		VALUES ('2024-01-01', 'success', 'full', 2, 2, 0, 1)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert files
	_, err = database.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, is_generated, parse_status, created_at, updated_at)
		VALUES ('main.go', 'go', 'abc', 100, 0, 'ok', '2024-01-01', '2024-01-01')`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert package
	_, err = database.Exec(`INSERT INTO packages (name, import_path, directory_path, language, created_at, updated_at)
		VALUES ('main', 'example.com/test', '.', 'go', '2024-01-01', '2024-01-01')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO package_files (package_id, file_id) VALUES (1, 1)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert symbols
	_, err = database.Exec(`INSERT INTO symbols (file_id, package_id, name, qualified_name, symbol_kind, visibility, stable_id, created_at, updated_at)
		VALUES (1, 1, 'main', 'main.main', 'entrypoint', 'unexported', 'go:main.main:entrypoint', '2024-01-01', '2024-01-01')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO symbols (file_id, package_id, name, qualified_name, symbol_kind, visibility, stable_id, created_at, updated_at)
		VALUES (1, 1, 'Run', 'main.Run', 'function', 'exported', 'go:main.Run:function', '2024-01-01', '2024-01-01')`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExportSummary(t *testing.T) {
	database := setupTestDB(t)
	seedTestData(t, database)

	result, err := ExportSummary(database, "/test/repo")
	if err != nil {
		t.Fatal(err)
	}

	if result.RepoRoot != "/test/repo" {
		t.Errorf("expected /test/repo, got %s", result.RepoRoot)
	}
	if len(result.Languages) == 0 {
		t.Error("expected languages")
	}
	if len(result.Packages) == 0 {
		t.Error("expected packages")
	}
	if len(result.Entrypoints) == 0 {
		t.Error("expected entrypoints")
	}

	// Verify it marshals to valid JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Error("invalid JSON output")
	}
}

func TestExportGraph(t *testing.T) {
	database := setupTestDB(t)
	seedTestData(t, database)

	result, err := ExportGraph(database)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) == 0 {
		t.Error("expected nodes")
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Error("invalid JSON output")
	}
}

func TestExportSymbols(t *testing.T) {
	database := setupTestDB(t)
	seedTestData(t, database)

	result, err := ExportSymbols(database)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 symbols, got %d", len(result))
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Error("invalid JSON output")
	}
}

func TestExportPackages(t *testing.T) {
	database := setupTestDB(t)
	seedTestData(t, database)

	result, err := ExportPackages(database)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 package, got %d", len(result))
	}
	if result[0].FileCount != 1 {
		t.Errorf("expected file_count=1, got %d", result[0].FileCount)
	}
}

func TestExportEmptyIndex(t *testing.T) {
	database := setupTestDB(t)

	// Summary should work on empty index
	summary, err := ExportSummary(database, "/empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Languages) != 0 {
		t.Error("expected empty languages")
	}
	if len(summary.Packages) != 0 {
		t.Error("expected empty packages")
	}

	// Graph should work on empty index
	graph, err := ExportGraph(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 0 {
		t.Error("expected empty nodes")
	}

	// Symbols
	syms, err := ExportSymbols(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 0 {
		t.Error("expected empty symbols")
	}

	// Diagnostics
	diags, err := ExportDiagnostics(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Error("expected empty diagnostics")
	}
}
