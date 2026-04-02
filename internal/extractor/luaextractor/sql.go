package luaextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var sqlKeywordRe = regexp.MustCompile(`(?i)\b(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|FROM|WHERE|JOIN|TABLE|INDEX)\b`)

func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Migration file detection
	if strings.Contains(filePath, "migration") {
		dataMap := map[string]string{
			"type": "migration",
			"file": filePath,
		}
		dataJSON, _ := json.Marshal(dataMap)

		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "migration",
			Name:         filePath,
			DataJSON:     string(dataJSON),
			Confidence:   "exact",
		})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "migrates",
			Confidence:    "exact",
			Line:          1,
			RawTargetText: filePath,
		})
	}

	// Scan each code line for SQL keywords.
	// Lua doesn't have string literal boundaries -- scan full line content per spec section 6.13.
	seen := map[int]bool{}
	for i, line := range lines {
		lineNum := i + 1
		if i >= len(codeLines) || !codeLines[i] {
			continue
		}
		if seen[lineNum] {
			continue
		}
		if sqlKeywordRe.MatchString(line) {
			seen[lineNum] = true

			query := strings.TrimSpace(line)
			if len(query) > 200 {
				query = query[:200]
			}

			rawTarget := strings.TrimSpace(line)
			if len(rawTarget) > 100 {
				rawTarget = rawTarget[:100]
			}

			dataMap := map[string]string{"query": query}
			dataJSON, _ := json.Marshal(dataMap)

			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "touches_table",
				Confidence:    "heuristic",
				Line:          lineNum,
				RawTargetText: rawTarget,
			})
			arts = append(arts, extractor.ArtifactRecord{
				ArtifactKind: "sql_query",
				Name:         query,
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})
		}
	}

	return refs, arts
}
