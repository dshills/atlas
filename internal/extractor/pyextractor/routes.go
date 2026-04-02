package pyextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	decoratorRouteRe = regexp.MustCompile(`(?m)@(\w+)\.(route|get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]`)
	djangoPathRe     = regexp.MustCompile(`(?m)(?:path|re_path|url)\s*\(\s*['"]([^'"]+)['"]`)
)

func extractRoutes(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Flask/FastAPI decorator routes
	for _, m := range decoratorRouteRe.FindAllStringSubmatchIndex(content, -1) {
		method := strings.ToUpper(content[m[4]:m[5]])
		if method == "ROUTE" {
			method = "ANY"
		}
		path := content[m[6]:m[7]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRouteRecords(method, path, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// Django path/re_path/url
	for _, m := range djangoPathRe.FindAllStringSubmatchIndex(content, -1) {
		path := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRouteRecords("ANY", path, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	return refs, arts
}

func buildRouteRecords(method, path string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	name := method + " " + path

	dataMap := map[string]string{
		"method":  method,
		"path":    path,
		"handler": "",
	}
	dataJSON, _ := json.Marshal(dataMap)

	ref := extractor.ReferenceRecord{
		ReferenceKind: "registers_route",
		Confidence:    "exact",
		Line:          line,
		RawTargetText: path,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "route",
		Name:         name,
		DataJSON:     string(dataJSON),
		Confidence:   "exact",
	}

	return ref, art
}
