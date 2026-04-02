package query

import (
	"database/sql"
)

// PackageResult represents a package query result.
type PackageResult struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	ImportPath    string `json:"importPath,omitempty"`
	DirectoryPath string `json:"directoryPath"`
	Language      string `json:"language"`
	FileCount     int    `json:"fileCount"`
	SymbolCount   int    `json:"symbolCount"`
}

// FindPackage searches for packages by name or import path.
func FindPackage(db *sql.DB, name string) ([]PackageResult, error) {
	query := `SELECT p.id, p.name, p.import_path, p.directory_path, p.language,
		(SELECT COUNT(*) FROM package_files pf WHERE pf.package_id = p.id) AS file_count,
		(SELECT COUNT(*) FROM symbols s WHERE s.package_id = p.id) AS symbol_count
		FROM packages p
		WHERE p.name = ? OR p.import_path = ?
		ORDER BY p.name`

	rows, err := db.Query(query, name, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []PackageResult
	for rows.Next() {
		var r PackageResult
		var importPath sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &importPath, &r.DirectoryPath, &r.Language, &r.FileCount, &r.SymbolCount); err != nil {
			return nil, err
		}
		if importPath.Valid {
			r.ImportPath = importPath.String
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ListPackages returns all packages.
func ListPackages(db *sql.DB) ([]PackageResult, error) {
	query := `SELECT p.id, p.name, p.import_path, p.directory_path, p.language,
		(SELECT COUNT(*) FROM package_files pf WHERE pf.package_id = p.id) AS file_count,
		(SELECT COUNT(*) FROM symbols s WHERE s.package_id = p.id) AS symbol_count
		FROM packages p
		ORDER BY p.name`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []PackageResult
	for rows.Next() {
		var r PackageResult
		var importPath sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &importPath, &r.DirectoryPath, &r.Language, &r.FileCount, &r.SymbolCount); err != nil {
			return nil, err
		}
		if importPath.Valid {
			r.ImportPath = importPath.String
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
