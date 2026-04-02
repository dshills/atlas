package pyextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	osEnvironBrackRe = regexp.MustCompile(`(?m)os\.environ\[['"]([^'"]+)['"]\]`)
	osEnvironGetRe   = regexp.MustCompile(`(?m)os\.environ\.get\s*\(\s*['"]([^'"]+)['"]`)
	osGetenvRe       = regexp.MustCompile(`(?m)os\.getenv\s*\(\s*['"]([^'"]+)['"]`)
	settingsDotRe    = regexp.MustCompile(`(?m)settings\.([A-Z_][A-Z0-9_]*)`)
)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// os.environ['KEY']
	for _, m := range osEnvironBrackRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "os.environ", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// os.environ.get('KEY')
	for _, m := range osEnvironGetRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "os.environ", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// os.getenv('KEY')
	for _, m := range osGetenvRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildConfigRecords(key, "os.getenv", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// settings.KEY
	for _, m := range settingsDotRe.FindAllStringSubmatchIndex(content, -1) {
		key := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"key":    key,
			"source": "settings",
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
