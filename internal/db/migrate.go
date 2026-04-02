package db

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate runs all pending migrations in order.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("creating schema_meta: %w", err)
	}

	applied, err := appliedVersion(db)
	if err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		ver := strings.TrimSuffix(name, ".sql")

		if ver <= applied {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("running migration %s: %w", name, err)
		}

		if _, err := db.Exec(`INSERT OR REPLACE INTO schema_meta (key, value) VALUES ('migration_version', ?)`, ver); err != nil {
			return fmt.Errorf("recording migration %s: %w", name, err)
		}
	}

	return nil
}

func appliedVersion(db *sql.DB) (string, error) {
	var ver string
	err := db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'migration_version'`).Scan(&ver)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return ver, err
}
