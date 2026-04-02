package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/goextractor"
)

func setupExtractorIndex(t *testing.T) (string, *Indexer) {
	t.Helper()
	dir, s := setupTestIndex(t)
	cfg := config.DefaultConfig()
	cfg.RepoRoot = dir

	idx := New(dir, cfg, s)
	reg := extractor.NewRegistry()
	reg.Register(goextractor.New())
	idx.Registry = reg
	idx.ModulePath = "example.com/test"

	return dir, idx
}

func TestIncrementalOnlyReindexesChanged(t *testing.T) {
	dir, idx := setupExtractorIndex(t)

	// First full run
	result, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesChanged != 2 {
		t.Fatalf("expected 2 files changed on initial, got %d", result.FilesChanged)
	}

	// Modify only one file
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() { println(\"modified\") }\n")

	idx2 := New(dir, idx.Config, idx.Store)
	idx2.Registry = idx.Registry
	idx2.ModulePath = idx.ModulePath

	result2, err := idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}
	if result2.FilesChanged != 1 {
		t.Errorf("expected 1 file changed on second run, got %d", result2.FilesChanged)
	}
}

func TestInvalidationCascade(t *testing.T) {
	dir, idx := setupExtractorIndex(t)

	// Full index
	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify symbols exist
	var symCount int
	err = idx.Store.DB.QueryRow(`SELECT COUNT(*) FROM symbols`).Scan(&symCount)
	if err != nil {
		t.Fatal(err)
	}
	if symCount == 0 {
		t.Fatal("expected symbols after initial index")
	}

	// Modify a file
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\n\nfunc newFunc() {}\n")

	idx2 := New(dir, idx.Config, idx.Store)
	idx2.Registry = idx.Registry
	idx2.ModulePath = idx.ModulePath

	_, err = idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify old symbols from modified file are gone, new ones exist
	var newFuncCount int
	err = idx.Store.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE name = 'newFunc'`).Scan(&newFuncCount)
	if err != nil {
		t.Fatal(err)
	}
	if newFuncCount != 1 {
		t.Errorf("expected 1 'newFunc' symbol, got %d", newFuncCount)
	}

	// Old main should be gone (it was an entrypoint, now just a function)
	var mainCount int
	err = idx.Store.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE name = 'main' AND symbol_kind = 'entrypoint'`).Scan(&mainCount)
	if err != nil {
		t.Fatal(err)
	}
	if mainCount != 0 {
		t.Errorf("expected 0 entrypoint 'main' symbols after modification, got %d", mainCount)
	}
}

func TestFileDeletionCascade(t *testing.T) {
	dir, idx := setupExtractorIndex(t)

	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Count symbols from pkg/util.go
	var beforeCount int
	err = idx.Store.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE qualified_name LIKE 'pkg.%'`).Scan(&beforeCount)
	if err != nil {
		t.Fatal(err)
	}
	if beforeCount == 0 {
		t.Fatal("expected symbols from pkg package")
	}

	// Delete the file
	if err := os.Remove(filepath.Join(dir, "pkg", "util.go")); err != nil {
		t.Fatal(err)
	}

	idx2 := New(dir, idx.Config, idx.Store)
	idx2.Registry = idx.Registry
	idx2.ModulePath = idx.ModulePath

	_, err = idx2.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify symbols from deleted file are gone
	var afterCount int
	err = idx.Store.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE qualified_name LIKE 'pkg.%'`).Scan(&afterCount)
	if err != nil {
		t.Fatal(err)
	}
	if afterCount != 0 {
		t.Errorf("expected 0 pkg symbols after deletion, got %d", afterCount)
	}
}

func TestReferenceResolution(t *testing.T) {
	dir, idx := setupExtractorIndex(t)

	// Create files that reference each other
	mustWrite(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

	_, err := idx.Run("full", "")
	if err != nil {
		t.Fatal(err)
	}

	// Check that import references exist
	var refCount int
	err = idx.Store.DB.QueryRow(`SELECT COUNT(*) FROM "references" WHERE reference_kind = 'imports'`).Scan(&refCount)
	if err != nil {
		t.Fatal(err)
	}
	if refCount == 0 {
		t.Error("expected import references")
	}
}

// Ensure the Extract function is called on context cancellation
func TestExtractWithContext(t *testing.T) {
	ext := goextractor.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := extractor.ExtractRequest{
		FilePath: "main.go",
		Content:  []byte("package main\n"),
	}

	// Should still work (go/parser doesn't check context)
	result, err := ext.Extract(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
