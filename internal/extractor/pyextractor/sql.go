package pyextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	sqlKeywordRe      = regexp.MustCompile(`(?i)^\s*(CREATE\s+TABLE|ALTER\s+TABLE|INSERT|SELECT|UPDATE|DELETE|DROP)`)
	pyStringLiteralRe = regexp.MustCompile("(?s)['\"]([^'\"]{20,})['\"]")
	pyTripleQuoteRe   = regexp.MustCompile("(?s)(?:\"\"\"(.*?)\"\"\"|'''(.*?)''')")
)

func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Migration file detection
	if strings.Contains(filePath, "migrations/") || strings.Contains(filePath, "migrate/") {
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

	// Collect SQL strings from triple-quoted strings
	for _, m := range pyTripleQuoteRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}

		// Group 1 is """, group 2 is '''
		var strContent string
		if m[2] >= 0 && m[3] >= 0 {
			strContent = content[m[2]:m[3]]
		} else if m[4] >= 0 && m[5] >= 0 {
			strContent = content[m[4]:m[5]]
		}
		if strContent == "" {
			continue
		}

		if sqlKeywordRe.MatchString(strContent) {
			r, a := buildSQLRecords(strContent, line)
			refs = append(refs, r)
			arts = append(arts, a)
		}
	}

	// Collect SQL strings from single/double quoted strings (20+ chars)
	for _, m := range pyStringLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		// Skip if this is inside a triple-quote region
		matchStart := m[0]
		if isInsideTripleQuote(content, matchStart) {
			continue
		}

		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}

		strContent := content[m[2]:m[3]]
		if sqlKeywordRe.MatchString(strContent) {
			r, a := buildSQLRecords(strContent, line)
			refs = append(refs, r)
			arts = append(arts, a)
		}
	}

	_ = lines // parameter kept for API consistency
	return refs, arts
}

func isInsideTripleQuote(content string, pos int) bool {
	for _, m := range pyTripleQuoteRe.FindAllStringIndex(content, -1) {
		if pos > m[0] && pos < m[1] {
			return true
		}
	}
	return false
}

func buildSQLRecords(query string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	truncatedQuery := query
	if len(truncatedQuery) > 200 {
		truncatedQuery = truncatedQuery[:200]
	}

	rawTarget := query
	if len(rawTarget) > 100 {
		rawTarget = rawTarget[:100]
	}

	dataMap := map[string]string{
		"query": truncatedQuery,
	}
	dataJSON, _ := json.Marshal(dataMap)

	ref := extractor.ReferenceRecord{
		ReferenceKind: "touches_table",
		Confidence:    "heuristic",
		Line:          line,
		RawTargetText: rawTarget,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "sql_query",
		Name:         truncatedQuery,
		DataJSON:     string(dataJSON),
		Confidence:   "heuristic",
	}

	return ref, art
}
