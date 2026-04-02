package rustextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	envVarRe        = regexp.MustCompile(`(?m)(?:^|[^a-zA-Z0-9_])(?:std::)?env::var(?:_os)?\s*\(\s*"([^"]+)"`)
	dotenvVarRe     = regexp.MustCompile(`(?m)(?:dotenv|dotenvy)::var\s*\(\s*"([^"]+)"`)
	rustConfigGetRe = regexp.MustCompile(`(?m)config\.get(?:_string|_int|_float|_bool|_array|_table)?(?:::<[^>]+>)?\s*\(\s*"([^"]+)"`)
)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// env::var("KEY") and std::env::var("KEY")
	for _, m := range envVarRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRustConfigRecords(key, "env::var", "env_var", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// dotenv::var("KEY") and dotenvy::var("KEY")
	for _, m := range dotenvVarRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRustConfigRecords(key, "dotenv::var", "env_var", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// config.get("key") variants
	for _, m := range rustConfigGetRe.FindAllStringSubmatchIndex(content, -1) {
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

func buildRustConfigRecords(key, source, artifactKind string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
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
		ArtifactKind: artifactKind,
		Name:         key,
		DataJSON:     string(dataJSON),
		Confidence:   "exact",
	}

	return ref, art
}
