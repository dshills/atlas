package luaextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	luaHTTPClientRe = regexp.MustCompile(`(?m)\b(http\.request|socket\.http)\b`)
	luaBgJobRe      = regexp.MustCompile(`(?m)\b(ngx\.timer\.at|copas\.addthread)\b`)
)

func extractServices(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// HTTP client patterns
	for _, m := range luaHTTPClientRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}

		dataMap := map[string]string{"name": name}
		dataJSON, _ := json.Marshal(dataMap)

		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "invokes_external_api",
			Confidence:    "heuristic",
			Line:          line,
			RawTargetText: name,
		})
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "external_service",
			Name:         name,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	// Background job patterns
	for _, m := range luaBgJobRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}

		dataMap := map[string]string{"name": name}
		dataJSON, _ := json.Marshal(dataMap)

		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "invokes_external_api",
			Confidence:    "heuristic",
			Line:          line,
			RawTargetText: name,
		})
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         name,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	return refs, arts
}
