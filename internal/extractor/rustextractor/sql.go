package rustextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	sqlKeywordRe        = regexp.MustCompile(`(?i)^\s*(CREATE\s+TABLE|ALTER\s+TABLE|INSERT|SELECT|UPDATE|DELETE|DROP)`)
	rustStringLiteralRe = regexp.MustCompile(`(?s)"([^"]{20,})"`)
	rustRawStringRe     = regexp.MustCompile(`(?s)r#*"(.*?)"#*`)
	sqlxQueryRe         = regexp.MustCompile(`(?m)(?:sqlx::)?query(?:_as)?!?\s*\(\s*r?#*"([^"]+)"`)
)

func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Migration file detection
	if strings.Contains(filePath, "migrations/") || strings.Contains(filePath, "migrate/") {
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

	// Collect SQL strings from various sources, deduplicating by byte offset.
	type sqlMatch struct {
		query string
		line  int
	}
	var candidates []sqlMatch
	seen := make(map[int]bool) // track start offsets to avoid duplicates

	// sqlx::query!("...") and variants (highest priority)
	for _, m := range sqlxQueryRe.FindAllStringSubmatchIndex(content, -1) {
		q := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		seen[m[2]] = true
		candidates = append(candidates, sqlMatch{query: q, line: line})
	}

	// Raw strings r"..." and r#"..."#
	for _, m := range rustRawStringRe.FindAllStringSubmatchIndex(content, -1) {
		if seen[m[2]] {
			continue
		}
		q := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		seen[m[2]] = true
		candidates = append(candidates, sqlMatch{query: q, line: line})
	}

	// Regular string literals (20+ chars)
	for _, m := range rustStringLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if seen[m[2]] {
			continue
		}
		q := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		candidates = append(candidates, sqlMatch{query: q, line: line})
	}

	for _, c := range candidates {
		if c.line-1 >= len(codeLines) || !codeLines[c.line-1] {
			continue
		}
		if !sqlKeywordRe.MatchString(c.query) {
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
