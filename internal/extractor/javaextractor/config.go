package javaextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	systemGetenvRe   = regexp.MustCompile(`(?m)System\.getenv\s*\(\s*"([^"]+)"`)
	systemPropertyRe = regexp.MustCompile(`(?m)System\.getProperty\s*\(\s*"([^"]+)"`)
	springValueRe    = regexp.MustCompile(`(?m)@Value\s*\(\s*"\$\{([^}]+)\}"`)
)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// System.getenv("KEY")
	for _, m := range systemGetenvRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "System.getenv", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// System.getProperty("key")
	for _, m := range systemPropertyRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "System.getProperty", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// @Value("${key}")
	for _, m := range springValueRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"key":    key,
			"source": "@Value",
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
