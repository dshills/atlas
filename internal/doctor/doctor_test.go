package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/atlas/internal/config"
	"github.com/dshills/atlas/internal/db"
	"github.com/dshills/atlas/internal/model"
)

func setupHealthyRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	storageDir := filepath.Join(dir, config.DefaultStorageDir)
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create DB with migrations
	dbPath := filepath.Join(storageDir, config.DefaultDBFile)
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	if err := db.InitMeta(database, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	_ = database.Close()

	// Create manifest
	manifest := model.Manifest{
		RepoRoot:         dir,
		SchemaVersion:    db.SchemaVersion,
		GeneratorVersion: "0.1.0",
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(storageDir, config.DefaultManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestDoctorHealthyRepo(t *testing.T) {
	dir := setupHealthyRepo(t)
	report := Run(dir)

	if !report.Healthy {
		for _, c := range report.Checks {
			if c.Status == "fail" {
				t.Errorf("check %s failed: %s", c.Name, c.Details)
			}
		}
		t.Fatal("expected healthy report")
	}
}

func TestDoctorMissingDB(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, config.DefaultStorageDir)
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No DB file created

	report := Run(dir)
	if report.Healthy {
		t.Error("expected unhealthy report with missing DB")
	}

	foundDBFail := false
	for _, c := range report.Checks {
		if c.Name == "database" && c.Status == "fail" {
			foundDBFail = true
		}
	}
	if !foundDBFail {
		t.Error("expected database check to fail")
	}
}

func TestDoctorManifestMismatch(t *testing.T) {
	dir := setupHealthyRepo(t)
	storageDir := filepath.Join(dir, config.DefaultStorageDir)

	// Overwrite manifest with wrong repo root
	manifest := model.Manifest{
		RepoRoot:      "/wrong/path",
		SchemaVersion: db.SchemaVersion,
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(storageDir, config.DefaultManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(dir)
	if report.Healthy {
		t.Error("expected unhealthy report with manifest mismatch")
	}

	for _, c := range report.Checks {
		if c.Name == "manifest" && c.Status != "fail" {
			t.Error("expected manifest check to fail")
		}
	}
}
