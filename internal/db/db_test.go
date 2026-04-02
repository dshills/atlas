package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("opening test DB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestOpenAndMigrate(t *testing.T) {
	d := testDB(t)

	if err := Migrate(d); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify all 12 tables exist
	tables := []string{
		"schema_meta", "files", "packages", "package_files",
		"symbols", "references", "file_summaries", "package_summaries",
		"symbol_summaries", "artifacts", "index_runs", "diagnostics",
	}
	for _, table := range tables {
		var name string
		err := d.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	d := testDB(t)

	if err := Migrate(d); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}
	if err := Migrate(d); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
}

func TestSchemaVersionCheck(t *testing.T) {
	d := testDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}

	// Fresh DB should pass
	if err := CheckSchemaVersion(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set a future version
	if err := SetMeta(d, "schema_version", "999"); err != nil {
		t.Fatal(err)
	}
	if err := CheckSchemaVersion(d); err == nil {
		t.Error("expected error for newer schema version")
	}
}

func TestInitMeta(t *testing.T) {
	d := testDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}

	if err := InitMeta(d, "0.1.0"); err != nil {
		t.Fatalf("init meta failed: %v", err)
	}

	ver, err := GetMeta(d, "schema_version")
	if err != nil {
		t.Fatal(err)
	}
	if ver != "1" {
		t.Errorf("expected schema_version 1, got %s", ver)
	}

	gen, err := GetMeta(d, "generator_version")
	if err != nil {
		t.Fatal(err)
	}
	if gen != "0.1.0" {
		t.Errorf("expected generator_version 0.1.0, got %s", gen)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	d := testDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}

	var fk int
	if err := d.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Error("expected foreign_keys to be enabled")
	}
}

func TestCheckConstraints(t *testing.T) {
	d := testDB(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}

	// Insert a file with an invalid parse_status should fail
	_, err := d.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, parse_status, created_at, updated_at)
		VALUES ('test.go', 'go', 'abc', 100, 'invalid_status', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')`)
	if err == nil {
		t.Error("expected CHECK constraint violation for invalid parse_status")
	}

	// Valid parse_status should work
	_, err = d.Exec(`INSERT INTO files (path, language, content_hash, size_bytes, parse_status, created_at, updated_at)
		VALUES ('test.go', 'go', 'abc', 100, 'ok', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Errorf("valid insert failed: %v", err)
	}
}
