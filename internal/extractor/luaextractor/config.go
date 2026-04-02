package luaextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var osGetenvRe = regexp.MustCompile(`(?m)os\.getenv\s*\(\s*["']([^"']+)["']`)

func extractConfigAccess(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	for _, m := range osGetenvRe.FindAllStringSubmatchIndex(content, -1) {
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

	return refs, arts
}
