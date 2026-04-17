package store

import (
	"database/sql"
	"time"
)

// Store provides persistence operations on top of the Atlas database.
type Store struct {
	DB *sql.DB
}

// New creates a new Store.
func New(db *sql.DB) *Store {
	return &Store{DB: db}
}

// FileRow represents a row in the files table.
type FileRow struct {
	ID              int64
	Path            string
	Language        string
	PackageName     sql.NullString
	ModuleName      sql.NullString
	ContentHash     string
	SizeBytes       int64
	LastModifiedUTC sql.NullString
	GitCommit       sql.NullString
	IsGenerated     bool
	ParseStatus     string
	CreatedAt       string
	UpdatedAt       string
}

// UpsertFile inserts or updates a file row using the provided Execer.
// Pass s.DB for auto-commit semantics, or a *sql.Tx to batch with other writes.
func (s *Store) UpsertFile(tx Execer, f *FileRow) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existingID int64
	err := tx.QueryRow(`SELECT id FROM files WHERE path = ?`, f.Path).Scan(&existingID)
	if err == sql.ErrNoRows {
		f.CreatedAt = now
		f.UpdatedAt = now
		res, err := tx.Exec(`INSERT INTO files (path, language, package_name, module_name, content_hash, size_bytes, last_modified_utc, git_commit, is_generated, parse_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.Path, f.Language, f.PackageName, f.ModuleName, f.ContentHash, f.SizeBytes, f.LastModifiedUTC, f.GitCommit,
			boolToInt(f.IsGenerated), f.ParseStatus, f.CreatedAt, f.UpdatedAt)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	if err != nil {
		return 0, err
	}

	f.UpdatedAt = now
	_, err = tx.Exec(`UPDATE files SET language=?, package_name=?, module_name=?, content_hash=?, size_bytes=?, last_modified_utc=?, git_commit=?, is_generated=?, parse_status=?, updated_at=?
		WHERE id=?`,
		f.Language, f.PackageName, f.ModuleName, f.ContentHash, f.SizeBytes, f.LastModifiedUTC, f.GitCommit,
		boolToInt(f.IsGenerated), f.ParseStatus, f.UpdatedAt, existingID)
	if err != nil {
		return 0, err
	}
	return existingID, nil
}

// SetParseStatus updates only the parse_status column for a file.
func (s *Store) SetParseStatus(tx Execer, fileID int64, status string) error {
	_, err := tx.Exec(`UPDATE files SET parse_status = ? WHERE id = ?`, status, fileID)
	return err
}

// GetFileByPath retrieves a file by path.
func (s *Store) GetFileByPath(path string) (*FileRow, error) {
	f := &FileRow{}
	var isGen int
	err := s.DB.QueryRow(`SELECT id, path, language, package_name, module_name, content_hash, size_bytes, last_modified_utc, git_commit, is_generated, parse_status, created_at, updated_at FROM files WHERE path = ?`, path).
		Scan(&f.ID, &f.Path, &f.Language, &f.PackageName, &f.ModuleName, &f.ContentHash, &f.SizeBytes, &f.LastModifiedUTC, &f.GitCommit, &isGen, &f.ParseStatus, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	f.IsGenerated = isGen == 1
	return f, nil
}

// DeleteFile removes a file row (cascades to symbols, references, etc).
func (s *Store) DeleteFile(fileID int64) error {
	_, err := s.DB.Exec(`DELETE FROM files WHERE id = ?`, fileID)
	return err
}

// AllFilePaths returns all file paths in the database.
func (s *Store) AllFilePaths() (map[string]int64, error) {
	rows, err := s.DB.Query(`SELECT id, path FROM files`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]int64)
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		result[path] = id
	}
	return result, rows.Err()
}

// FileHashMap returns a map of path → content_hash for all files.
func (s *Store) FileHashMap() (map[string]string, error) {
	rows, err := s.DB.Query(`SELECT path, content_hash FROM files`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		result[path] = hash
	}
	return result, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
