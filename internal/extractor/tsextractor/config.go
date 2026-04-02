package tsextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	processEnvDotRe   = regexp.MustCompile(`(?m)process\.env\.([A-Z_][A-Z0-9_]*)`)
	processEnvBrackRe = regexp.MustCompile(`(?m)process\.env\[['"]([^'"]+)['"]\]`)
	configGetRe       = regexp.MustCompile(`(?m)config\.get(?:<[^>]+>)?\s*\(\s*['"]([^'"]+)['"]`)
)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// process.env.KEY
	for _, m := range processEnvDotRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "process.env", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// process.env['KEY']
	for _, m := range processEnvBrackRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "process.env", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// config.get('key')
	for _, m := range configGetRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"key":    key,
			"source": "config.get",
		}
		dataJSON, _ := json.Marshal(dataMap)

		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "uses_config",
			Confidence:    "exact",
			Line:          line,
			RawTargetText: key,
		})
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "config_key",
			Name:         key,
			DataJSON:     string(dataJSON),
			Confidence:   "exact",
		})
	}

	return refs, arts
}

func buildConfigRecords(key, source string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	dataMap := map[string]string{
		"key":    key,
		"source": source,
	}
	dataJSON, _ := json.Marshal(dataMap)

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
