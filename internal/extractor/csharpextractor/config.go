package csharpextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	envGetRe        = regexp.MustCompile(`(?m)Environment\.GetEnvironmentVariable\s*\(\s*"([^"]+)"`)
	configGetRe     = regexp.MustCompile(`(?m)(?:configuration|config)\[["']([^"']+)["']\]`)
	configGetSectRe = regexp.MustCompile(`(?m)(?:configuration|config)\.GetSection\s*\(\s*"([^"]+)"`)
	configGetValRe  = regexp.MustCompile(`(?m)(?:configuration|config)\.GetValue(?:<[^>]+>)?\s*\(\s*"([^"]+)"`)
)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Environment.GetEnvironmentVariable("KEY")
	for _, m := range envGetRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{"name": key}
		dataJSON, _ := json.Marshal(dataMap)

		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "uses_config",
			Confidence:    "exact",
			Line:          line,
			RawTargetText: key,
		})
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "env_var",
			Name:         key,
			DataJSON:     string(dataJSON),
			Confidence:   "exact",
		})
	}

	// configuration["key"] or config['key']
	for _, m := range configGetRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigKeyRecords(key, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// configuration.GetSection("key")
	for _, m := range configGetSectRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigKeyRecords(key, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// configuration.GetValue<T>("key")
	for _, m := range configGetValRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigKeyRecords(key, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	return refs, arts
}

func buildConfigKeyRecords(key string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	dataMap := map[string]string{"key": key}
	dataJSON, _ := json.Marshal(dataMap)

	ref := extractor.ReferenceRecord{
		ReferenceKind: "uses_config",
		Confidence:    "likely",
		Line:          line,
		RawTargetText: key,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "config_key",
		Name:         key,
		DataJSON:     string(dataJSON),
		Confidence:   "likely",
	}

	return ref, art
}
