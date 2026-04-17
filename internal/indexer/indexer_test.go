package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/db"
	"github.com/dshills/atlas/internal/store"
)

func setupTestIndex(t *testing.T) (string, *store.Store) {
	t.Helper()
	dir := t.TempDir()

	// Create fixture Go files
	mustMkdir(t, filepath.Join(dir, "pkg"))
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	mustWrite(t, filepath.Join(dir, "pkg", "util.go"), "package pkg\n\nfunc Helper() {}\n")

	dbPath := filepath.Join(dir, ".atlas", "atlas.db")
	mustMkdir(t, filepath.Join(dir, ".atlas"))
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	return dir, store.New(database)
}

func TestIndexFullRun(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir

	idx := New(dir, cfg, s)
	result, err := idx.Run("full", "")
	if err != nil {
		t.Fatalf("index run failed: %v", err)
	}

	if result.FilesScanned != 2 {
		t.Errorf("expected 2 files scanned, got %d", result.FilesScanned)
	}
	if result.FilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d", result.FilesChanged)
	}
	if result.Status != "success" {
		t.Errorf("expected success status, got %s", result.Status)
	}
}

func TestIndexUnchangedFiles(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir

	idx := New(dir, cfg, s)

	// First run
	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Second run — nothing changed
	idx2 := New(dir, cfg, s)
	result, err := idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesChanged != 0 {
		t.Errorf("expected 0 files changed on re-run, got %d", result.FilesChanged)
	}
}

func TestIndexFileModification(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir

	idx := New(dir, cfg, s)
	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Modify a file
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() { println(\"hi\") }\n")

	idx2 := New(dir, cfg, s)
	result, err := idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesChanged != 1 {
		t.Errorf("expected 1 file changed, got %d", result.FilesChanged)
	}
}

func TestIndexFileDeletion(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir

	idx := New(dir, cfg, s)
	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Delete a file
	if err := os.Remove(filepath.Join(dir, "pkg", "util.go")); err != nil {
		t.Fatal(err)
	}

	idx2 := New(dir, cfg, s)
	result, err := idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesDeleted != 1 {
		t.Errorf("expected 1 file deleted, got %d", result.FilesDeleted)
	}

	// Verify it's removed from DB
	_, err = s.GetFileByPath("pkg/util.go")
	if err == nil {
		t.Error("expected file to be deleted from DB")
	}
}

func TestReindexClearsAll(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir

	idx := New(dir, cfg, s)
	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Clear and re-run
	if err := idx.ClearAll(); err != nil {
		t.Fatal(err)
	}

	idx2 := New(dir, cfg, s)
	result, err := idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesNew != 2 {
		t.Errorf("expected 2 new files after reindex, got %d", result.FilesNew)
	}
}

func TestBatchSizeOneStillWorks(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir
	cfg.Indexing.BatchSize = 1 // each file is its own tx

	idx := New(dir, cfg, s)
	result, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d", result.FilesChanged)
	}
}

func TestBatchSizeLargerThanWork(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir
	cfg.Indexing.BatchSize = 1000 // single flush at end

	idx := New(dir, cfg, s)
	result, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d", result.FilesChanged)
	}
}

func TestBatchDefaultWhenZero(t *testing.T) {
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir
	cfg.Indexing.BatchSize = 0 // should fall back to DefaultBatchSize

	idx := New(dir, cfg, s)
	if _, err := idx.Run("full", ""); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerCountDefaultsToCPU(t *testing.T) {
	_, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.Indexing.Workers = 0

	idx := New(t.TempDir(), cfg, s)
	if idx.workerCount() < 1 {
		t.Errorf("workerCount = %d, want >= 1", idx.workerCount())
	}
}

func TestWorkerCountExplicit(t *testing.T) {
	_, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.Indexing.Workers = 3

	idx := New(t.TempDir(), cfg, s)
	if got := idx.workerCount(); got != 3 {
		t.Errorf("workerCount = %d, want 3", got)
	}
}

func TestLoadSymbolIDsForFilesChunks(t *testing.T) {
	_, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	idx := New(t.TempDir(), cfg, s)

	// Insert a small set of files and symbols directly.
	var fileIDs []int64
	for i := 0; i < 5; i++ {
		path := "f" + string(rune('A'+i)) + ".go"
		id, err := s.UpsertFile(s.DB, sampleFileRow(path))
		if err != nil {
			t.Fatal(err)
		}
		fileIDs = append(fileIDs, id)
		if _, err := s.DB.Exec(
			`INSERT INTO symbols (file_id, name, qualified_name, symbol_kind, visibility, stable_id, created_at, updated_at)
			 VALUES (?, ?, ?, 'function', 'exported', ?, '2024-01-01', '2024-01-01')`,
			id, "f"+string(rune('A'+i)), "pkg.f"+string(rune('A'+i)), "pkg.f"+string(rune('A'+i))+"#1",
		); err != nil {
			t.Fatal(err)
		}
	}

	got, err := idx.loadSymbolIDsForFiles(fileIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Errorf("expected 5 symbol ids, got %d", len(got))
	}
}

func TestLoadSymbolIDsEmpty(t *testing.T) {
	_, s := setupTestIndex(t)
	idx := New(t.TempDir(), config.DefaultConfig(), s)

	got, err := idx.loadSymbolIDsForFiles(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 ids for empty input, got %d", len(got))
	}
}

func sampleFileRow(path string) *store.FileRow {
	return &store.FileRow{
		Path: path, Language: "go", ContentHash: "x", SizeBytes: 1, ParseStatus: "ok",
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
