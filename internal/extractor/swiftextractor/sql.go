package swiftextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	sqlKeywordRe  = regexp.MustCompile(`(?i)\b(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP|FROM|WHERE|JOIN|TABLE|INDEX)\b`)
	nsPredicateRe = regexp.MustCompile(`(?m)NSPredicate\s*\(\s*format\s*:\s*"([^"]+)"`)
	grdbFilterRe  = regexp.MustCompile(`(?m)\.filter\s*\(\s*sql\s*:\s*"([^"]+)"`)
)

func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Migration file detection
	if strings.Contains(filePath, "Migration") || strings.Contains(filePath, "migration") {
		dataJSON, _ := json.Marshal(map[string]string{
			"file": filePath,
			"type": "migration",
		})
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
			RawTargetText: truncate(filePath, 100),
		})
	}

	// Collect SQL candidates, deduplicating by line
	type sqlMatch struct {
		query string
		line  int
	}
	var candidates []sqlMatch
	seenLines := make(map[int]bool)

	// NSPredicate(format: "...")
	for _, m := range nsPredicateRe.FindAllStringSubmatchIndex(content, -1) {
		q := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if !seenLines[line] {
			seenLines[line] = true
			candidates = append(candidates, sqlMatch{query: q, line: line})
		}
	}

	// .filter(sql: "...")
	for _, m := range grdbFilterRe.FindAllStringSubmatchIndex(content, -1) {
		q := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if !seenLines[line] {
			seenLines[line] = true
			candidates = append(candidates, sqlMatch{query: q, line: line})
		}
	}

	// Scan code lines for SQL keywords in string literals
	for i, line := range lines {
		lineNum := i + 1
		if seenLines[lineNum] {
			continue
		}
		if lineNum-1 >= len(codeLines) || !codeLines[lineNum-1] {
			continue
		}
		if sqlKeywordRe.MatchString(line) && strings.Contains(line, `"`) {
			candidates = append(candidates, sqlMatch{query: strings.TrimSpace(line), line: lineNum})
			seenLines[lineNum] = true
		}
	}

	for _, c := range candidates {
		if c.line-1 >= len(codeLines) || !codeLines[c.line-1] {
			continue
		}

		truncatedQuery := truncate(c.query, 200)
		dataJSON, _ := json.Marshal(map[string]string{
			"query": truncatedQuery,
		})

		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "sql_query",
			Name:         truncatedQuery,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "touches_table",
			Confidence:    "heuristic",
			Line:          c.line,
			RawTargetText: truncate(c.query, 100),
		})
	}

	return refs, arts
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
