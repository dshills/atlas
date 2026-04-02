package csharpextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	aspnetRouteRe  = regexp.MustCompile(`(?m)\[(Http(?:Get|Post|Put|Delete|Patch|Head|Options))\s*\(\s*"([^"]*)"`)
	aspnetRouteRe2 = regexp.MustCompile(`(?m)\[Route\s*\(\s*"([^"]+)"`)
	minimalApiRe   = regexp.MustCompile(`(?m)app\.Map(Get|Post|Put|Delete|Patch)\s*\(\s*"([^"]+)"`)
)

func extractRoutes(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// ASP.NET attribute routes: [HttpGet("/path")]
	for _, m := range aspnetRouteRe.FindAllStringSubmatchIndex(content, -1) {
		attr := content[m[2]:m[3]] // e.g. "HttpGet"
		path := content[m[4]:m[5]] // e.g. "/users/{id}"
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		// Strip "Http" prefix and uppercase the rest
		method := strings.ToUpper(strings.TrimPrefix(attr, "Http"))
		r, a := buildRouteRecords(method, path, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// [Route("path")] attribute
	for _, m := range aspnetRouteRe2.FindAllStringSubmatchIndex(content, -1) {
		path := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRouteRecords("ANY", path, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// Minimal API routes: app.MapGet("/path", ...)
	for _, m := range minimalApiRe.FindAllStringSubmatchIndex(content, -1) {
		verb := content[m[2]:m[3]] // e.g. "Get"
		path := content[m[4]:m[5]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		method := strings.ToUpper(verb)
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
