package export

import (
	"database/sql"
	"fmt"
)

// PackageExport represents an exported package.
type PackageExport struct {
	Name          string `json:"name"`
	ImportPath    string `json:"import_path"`
	DirectoryPath string `json:"directory_path"`
	Language      string `json:"language"`
	FileCount     int    `json:"file_count"`
}

// ExportPackages produces all packages from the database.
func ExportPackages(database *sql.DB) ([]PackageExport, error) {
	rows, err := database.Query(`SELECT p.name, COALESCE(p.import_path,''), p.directory_path, p.language, COUNT(pf.file_id)
		FROM packages p LEFT JOIN package_files pf ON p.id = pf.package_id GROUP BY p.id ORDER BY p.name`)
	if err != nil {
		return nil, fmt.Errorf("querying packages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := []PackageExport{}
	for rows.Next() {
		var p PackageExport
		if err := rows.Scan(&p.Name, &p.ImportPath, &p.DirectoryPath, &p.Language, &p.FileCount); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
