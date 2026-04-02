package swiftextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var vaporRouteRe = regexp.MustCompile(`(?m)(?:app|router|routes)\.(get|post|put|delete|patch)\s*\(\s*"([^"]+)"`)

func extractRoutes(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	for _, m := range vaporRouteRe.FindAllStringSubmatchIndex(content, -1) {
		method := strings.ToUpper(content[m[2]:m[3]])
		path := content[m[4]:m[5]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRouteRecords(method, path, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	return refs, arts
}

func buildRouteRecords(method, path string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	dataMap := map[string]string{"method": method, "path": path, "handler": ""}
	dataJSON, _ := json.Marshal(dataMap)

	ref := extractor.ReferenceRecord{
		ReferenceKind: "registers_route",
		Confidence:    "likely",
		Line:          line,
		RawTargetText: method + " " + path,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "route",
		Name:         method + " " + path,
		DataJSON:     string(dataJSON),
		Confidence:   "likely",
	}

	return ref, art
}
