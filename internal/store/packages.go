package store

import (
	"database/sql"
	"time"
)

// PackageRow represents a row in the packages table.
type PackageRow struct {
	ID            int64
	Name          string
	ImportPath    sql.NullString
	DirectoryPath string
	Language      string
	CreatedAt     string
	UpdatedAt     string
}

// UpsertPackage inserts or updates a package by directory_path.
func (s *Store) UpsertPackage(p *PackageRow) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existingID int64
	err := s.DB.QueryRow(`SELECT id FROM packages WHERE directory_path = ?`, p.DirectoryPath).Scan(&existingID)
	if err == sql.ErrNoRows {
		p.CreatedAt = now
		p.UpdatedAt = now
		res, err := s.DB.Exec(`INSERT INTO packages (name, import_path, directory_path, language, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			p.Name, p.ImportPath, p.DirectoryPath, p.Language, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	if err != nil {
		return 0, err
	}

	p.UpdatedAt = now
	_, err = s.DB.Exec(`UPDATE packages SET name=?, import_path=?, language=?, updated_at=? WHERE id=?`,
		p.Name, p.ImportPath, p.Language, p.UpdatedAt, existingID)
	return existingID, err
}

// LinkFileToPackage creates a package_files entry.
func (s *Store) LinkFileToPackage(fileID, packageID int64) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO package_files (package_id, file_id) VALUES (?, ?)`, packageID, fileID)
	return err
}

// UnlinkFileFromPackages removes all package_files entries for a file.
func (s *Store) UnlinkFileFromPackages(fileID int64) error {
	_, err := s.DB.Exec(`DELETE FROM package_files WHERE file_id = ?`, fileID)
	return err
}
