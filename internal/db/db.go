package db

import (
	"database/sql"
	"fmt"
	"strconv"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 1

// Open opens (or creates) a SQLite database at the given path with required pragmas.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

// SetMeta sets a key-value pair in schema_meta.
func SetMeta(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO schema_meta (key, value) VALUES (?, ?)`, key, value)
	return err
}

// GetMeta retrieves a value from schema_meta.
func GetMeta(db *sql.DB, key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM schema_meta WHERE key = ?`, key).Scan(&val)
	return val, err
}

// CheckSchemaVersion verifies the DB schema version is compatible with this binary.
func CheckSchemaVersion(d *sql.DB) error {
	ver, err := GetMeta(d, "schema_version")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // fresh DB
		}
		return err
	}

	v, err := strconv.Atoi(ver)
	if err != nil {
		return fmt.Errorf("invalid schema_version %q: %w", ver, err)
	}

	if v > SchemaVersion {
		return fmt.Errorf("database schema version %d is newer than binary schema version %d; upgrade Atlas", v, SchemaVersion)
	}

	return nil
}

// InitMeta writes initial schema_meta entries.
func InitMeta(d *sql.DB, generatorVersion string) error {
	if err := SetMeta(d, "schema_version", strconv.Itoa(SchemaVersion)); err != nil {
		return err
	}
	return SetMeta(d, "generator_version", generatorVersion)
}
