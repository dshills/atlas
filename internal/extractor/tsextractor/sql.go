package tsextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	sqlKeywordRe    = regexp.MustCompile(`(?i)^\s*(CREATE\s+TABLE|ALTER\s+TABLE|INSERT|SELECT|UPDATE|DELETE|DROP)`)
	stringLiteralRe = regexp.MustCompile("(?s)['\"]([^'\"]{20,})['\"]")
	templateLitRe   = regexp.MustCompile("(?s)`([^`]{20,})`")
)

func extractSQLArtifacts(content string, _ []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Migration file detection
	if strings.Contains(filePath, "migrations/") || strings.Contains(filePath, "migrate/") {
		dataMap := map[string]string{
			"file": filePath,
			"type": "migration",
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
			RawTargetText: filePath,
			Confidence:    "exact",
		})
	}

	// Extract SQL from string literals
	type literalMatch struct {
		value string
		line  int
	}
	var literals []literalMatch

	for _, m := range stringLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		literals = append(literals, literalMatch{
			value: content[m[2]:m[3]],
			line:  line,
		})
	}

	for _, m := range templateLitRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		literals = append(literals, literalMatch{
			value: content[m[2]:m[3]],
			line:  line,
		})
	}

	for _, lit := range literals {
		kw := sqlKeywordRe.FindStringSubmatch(lit.value)
		if kw == nil {
			continue
		}
		sqlKind := strings.ToUpper(strings.Fields(kw[1])[0])
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(kw[1])), "CREATE") {
			sqlKind = "CREATE TABLE"
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(kw[1])), "ALTER") {
			sqlKind = "ALTER TABLE"
		}

		queryTrunc := truncate(lit.value, 200)
		dataMap := map[string]string{
			"query": queryTrunc,
			"type":  sqlKind,
		}
		dataJSON, _ := json.Marshal(dataMap)

		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "sql_query",
			Name:         sqlKind,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "touches_table",
			RawTargetText: truncate(lit.value, 100),
			Confidence:    "heuristic",
			Line:          lit.line,
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
