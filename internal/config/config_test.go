package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.StorageDir != ".atlas" {
		t.Errorf("expected storage dir .atlas, got %s", cfg.StorageDir)
	}
	if !cfg.Languages.Go {
		t.Error("expected Go enabled by default")
	}
	if !cfg.Languages.Java {
		t.Error("expected Java enabled by default")
	}
	if cfg.Indexing.MaxFileSizeBytes != 1<<20 {
		t.Errorf("expected max file size 1MiB, got %d", cfg.Indexing.MaxFileSizeBytes)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected defaults when file missing, got version %d", cfg.Version)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: 2
languages:
  go: true
  typescript: false
  javascript: false
indexing:
  max_file_size_bytes: 2097152
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != 2 {
		t.Errorf("expected version 2, got %d", cfg.Version)
	}
	if cfg.Languages.TypeScript {
		t.Error("expected TypeScript disabled")
	}
	if cfg.Indexing.MaxFileSizeBytes != 2097152 {
		t.Errorf("expected 2MiB, got %d", cfg.Indexing.MaxFileSizeBytes)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
