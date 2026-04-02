package swiftextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	envGetRe  = regexp.MustCompile(`(?m)ProcessInfo\.processInfo\.environment\[["']([^"']+)["']\]`)
	envGetRe2 = regexp.MustCompile(`(?m)Environment\.get\s*\(\s*"([^"]+)"`)
)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// ProcessInfo.processInfo.environment["KEY"]
	for _, m := range envGetRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// Environment.get("KEY")
	for _, m := range envGetRe2.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	return refs, arts
}

func buildConfigRecords(key string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	dataJSON, _ := json.Marshal(map[string]string{"name": key})

	ref := extractor.ReferenceRecord{
		ReferenceKind: "uses_config",
		Confidence:    "exact",
		Line:          line,
		RawTargetText: key,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "env_var",
		Name:         key,
		DataJSON:     string(dataJSON),
		Confidence:   "exact",
	}

	return ref, art
}
