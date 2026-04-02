package query

import (
	"database/sql"

	"github.com/bmatcuk/doublestar/v4"
)

// FileResult represents a file query result.
type FileResult struct {
	ID          int64  `json:"id"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	PackageName string `json:"packageName,omitempty"`
	ContentHash string `json:"contentHash"`
	SizeBytes   int64  `json:"sizeBytes"`
	ParseStatus string `json:"parseStatus"`
	UpdatedAt   string `json:"updatedAt"`
}

// FileOptions controls file query filtering.
type FileOptions struct {
	Include []string // glob patterns to include
	Exclude []string // glob patterns to exclude
}

// FindFile searches for files by path. If exact is true, the path must match exactly.
// Otherwise, it performs a substring match.
func FindFile(db *sql.DB, pattern string, exact bool, opts FileOptions) ([]FileResult, error) {
	var query string
	var args []interface{}

	if exact {
		query = `SELECT id, path, language, package_name, content_hash, size_bytes, parse_status, updated_at
			FROM files WHERE path = ?`
		args = append(args, pattern)
	} else {
		query = `SELECT id, path, language, package_name, content_hash, size_bytes, parse_status, updated_at
			FROM files WHERE path LIKE ?`
		args = append(args, "%"+pattern+"%")
	}

	query += ` ORDER BY path`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []FileResult
	for rows.Next() {
		var r FileResult
		var pkgName sql.NullString
		if err := rows.Scan(&r.ID, &r.Path, &r.Language, &pkgName, &r.ContentHash, &r.SizeBytes, &r.ParseStatus, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if pkgName.Valid {
			r.PackageName = pkgName.String
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Apply include/exclude glob filters
	if len(opts.Include) > 0 || len(opts.Exclude) > 0 {
		results = filterFiles(results, opts)
	}

	return results, nil
}

func filterFiles(results []FileResult, opts FileOptions) []FileResult {
	var filtered []FileResult
	for _, r := range results {
		if !matchesInclude(r.Path, opts.Include) {
			continue
		}
		if matchesExclude(r.Path, opts.Exclude) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func matchesInclude(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if matched, _ := doublestar.Match(p, path); matched {
			return true
		}
	}
	return false
}

func matchesExclude(path string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := doublestar.Match(p, path); matched {
			return true
		}
	}
	return false
}
