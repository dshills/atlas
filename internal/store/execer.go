package store

import "database/sql"

// Execer is the subset of *sql.DB and *sql.Tx that the store's write methods
// need. Accepting it lets callers batch many inserts into one transaction
// (critical for SQLite, where each auto-commit statement costs an fsync).
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}
