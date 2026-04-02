package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// RouteExport represents an exported route artifact.
type RouteExport struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Handler    string `json:"handler"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Confidence string `json:"confidence"`
}

// ExportRoutes produces all route artifacts from the database.
func ExportRoutes(database *sql.DB) ([]RouteExport, error) {
	rows, err := database.Query(`SELECT a.data_json, f.path, COALESCE(s.start_line, 0), a.confidence
		FROM artifacts a
		JOIN files f ON a.file_id = f.id
		LEFT JOIN symbols s ON a.symbol_id = s.id
		WHERE a.artifact_kind = 'route'
		ORDER BY f.path`)
	if err != nil {
		return nil, fmt.Errorf("querying routes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := []RouteExport{}
	for rows.Next() {
		var dataJSON, file, confidence string
		var line int
		if err := rows.Scan(&dataJSON, &file, &line, &confidence); err != nil {
			return nil, err
		}

		var data map[string]string
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			continue
		}

		result = append(result, RouteExport{
			Method:     data["method"],
			Path:       data["path"],
			Handler:    data["handler"],
			File:       file,
			Line:       line,
			Confidence: confidence,
		})
	}
	return result, rows.Err()
}
