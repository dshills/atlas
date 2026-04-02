package query

import (
	"database/sql"
)

// DiagnosticResult represents a diagnostic query result.
type DiagnosticResult struct {
	ID          int64  `json:"id"`
	RunID       int64  `json:"runId"`
	FilePath    string `json:"filePath,omitempty"`
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Line        *int64 `json:"line,omitempty"`
	ColumnStart *int64 `json:"columnStart,omitempty"`
	ColumnEnd   *int64 `json:"columnEnd,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// ListDiagnostics returns diagnostics from the latest index run.
func ListDiagnostics(db *sql.DB) ([]DiagnosticResult, error) {
	query := `SELECT d.id, d.run_id, COALESCE(f.path, ''), d.severity, d.code, d.message,
		d.line, d.column_start, d.column_end, d.created_at
		FROM diagnostics d
		LEFT JOIN files f ON d.file_id = f.id
		WHERE d.run_id = (SELECT MAX(id) FROM index_runs)
		ORDER BY d.severity, f.path, d.line`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []DiagnosticResult
	for rows.Next() {
		var r DiagnosticResult
		var line, colStart, colEnd sql.NullInt64
		if err := rows.Scan(&r.ID, &r.RunID, &r.FilePath, &r.Severity, &r.Code, &r.Message,
			&line, &colStart, &colEnd, &r.CreatedAt); err != nil {
			return nil, err
		}
		if line.Valid {
			v := line.Int64
			r.Line = &v
		}
		if colStart.Valid {
			v := colStart.Int64
			r.ColumnStart = &v
		}
		if colEnd.Valid {
			v := colEnd.Int64
			r.ColumnEnd = &v
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
