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
