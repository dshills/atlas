package export

import (
	"database/sql"
	"fmt"
)

// DiagnosticExport represents an exported diagnostic.
type DiagnosticExport struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// ExportDiagnostics produces diagnostics from the most recent run.
func ExportDiagnostics(database *sql.DB) ([]DiagnosticExport, error) {
	// Find latest run
	var runID int64
	err := database.QueryRow(`SELECT id FROM index_runs ORDER BY id DESC LIMIT 1`).Scan(&runID)
	if err == sql.ErrNoRows {
		return []DiagnosticExport{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying latest run: %w", err)
	}

	rows, err := database.Query(`SELECT d.severity, d.code, d.message, COALESCE(f.path,''), COALESCE(d.line, 0)
		FROM diagnostics d
		LEFT JOIN files f ON d.file_id = f.id
		WHERE d.run_id = ?
		ORDER BY d.id`, runID)
	if err != nil {
		return nil, fmt.Errorf("querying diagnostics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := []DiagnosticExport{}
	for rows.Next() {
		var d DiagnosticExport
		if err := rows.Scan(&d.Severity, &d.Code, &d.Message, &d.File, &d.Line); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
