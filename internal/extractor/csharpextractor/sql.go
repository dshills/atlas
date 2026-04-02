package csharpextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	sqlKeywordRe = regexp.MustCompile(`(?i)\b(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|FROM|WHERE|JOIN|TABLE|INDEX)\b`)
	efCoreSqlRe  = regexp.MustCompile(`(?m)FromSqlRaw\s*\(\s*["']([^"']+)["']`)
)

func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Migration file detection
	if strings.Contains(filePath, "Migration") || strings.Contains(filePath, "migration") {
		dataMap := map[string]string{
			"file": filePath,
			"type": "migration",
		}
		dataJSON, _ := json.Marshal(dataMap)

		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "migration",
			Name:         filePath,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "migrates",
			RawTargetText: filePath,
			Confidence:    "heuristic",
		})
	}

	seen := make(map[int]bool)

	// Scan each code line for SQL keywords
	for i, line := range lines {
		if i >= len(codeLines) || !codeLines[i] {
			continue
		}
		if seen[i+1] {
			continue
		}
		if sqlKeywordRe.MatchString(line) {
			seen[i+1] = true
			queryTrunc := truncate(strings.TrimSpace(line), 200)
			dataMap := map[string]string{
				"query": queryTrunc,
				"type":  "sql",
			}
			dataJSON, _ := json.Marshal(dataMap)

			arts = append(arts, extractor.ArtifactRecord{
				ArtifactKind: "sql_query",
				Name:         "sql",
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})
			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "touches_table",
				RawTargetText: queryTrunc,
				Confidence:    "heuristic",
				Line:          i + 1,
			})
		}
	}

	// EF Core FromSqlRaw
	for _, m := range efCoreSqlRe.FindAllStringSubmatchIndex(content, -1) {
		sqlStr := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true

		queryTrunc := truncate(sqlStr, 200)
		dataMap := map[string]string{
			"query": queryTrunc,
			"type":  "ef_core",
		}
		dataJSON, _ := json.Marshal(dataMap)

		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "sql_query",
			Name:         "sql",
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "touches_table",
			RawTargetText: queryTrunc,
			Confidence:    "heuristic",
			Line:          line,
		})
	}

	return refs, arts
}

func truncate(s string, limit int) string {
	if len(s) > limit {
		return s[:limit]
	}
	return s
}
