package goextractor

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// extractSQLArtifacts detects SQL queries and migration files.
func extractSQLArtifacts(fset *token.FileSet, file *ast.File, filePath string) ([]extractor.ArtifactRecord, []extractor.ReferenceRecord) {
	var artifacts []extractor.ArtifactRecord
	var refs []extractor.ReferenceRecord

	// Check if this is a migration file
	if strings.Contains(filePath, "migrations/") || strings.Contains(filePath, "migrate/") {
		dataJSON, _ := json.Marshal(map[string]string{
			"file": filePath,
			"type": "migration",
		})
		artifacts = append(artifacts, extractor.ArtifactRecord{
			ArtifactKind: "migration",
			Name:         filePath,
			DataJSON:     string(dataJSON),
			Confidence:   "exact",
		})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "migrates",
			Confidence:    "exact",
			RawTargetText: filePath,
		})
	}

	// Scan string literals for SQL
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}

		val := strings.Trim(lit.Value, "`\"")
		upper := strings.ToUpper(strings.TrimSpace(val))

		var sqlKind string
		switch {
		case strings.HasPrefix(upper, "CREATE TABLE"):
			sqlKind = "CREATE TABLE"
		case strings.HasPrefix(upper, "ALTER TABLE"):
			sqlKind = "ALTER TABLE"
		case strings.HasPrefix(upper, "INSERT"):
			sqlKind = "INSERT"
		case strings.HasPrefix(upper, "SELECT"):
			sqlKind = "SELECT"
		default:
			return true
		}

		dataJSON, _ := json.Marshal(map[string]string{
			"query": truncate(val, 200),
			"type":  sqlKind,
		})

		artifacts = append(artifacts, extractor.ArtifactRecord{
			ArtifactKind: "sql_query",
			Name:         sqlKind,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})

		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "touches_table",
			Confidence:    "heuristic",
			RawTargetText: truncate(val, 100),
			Line:          fset.Position(lit.Pos()).Line,
		})

		return true
	})

	return artifacts, refs
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
