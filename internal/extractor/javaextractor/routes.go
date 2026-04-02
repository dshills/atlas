package javaextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	springMappingRe = regexp.MustCompile(`(?m)@(Get|Post|Put|Delete|Patch|Request)Mapping\s*\(\s*(?:value\s*=\s*)?["']([^"']+)["']`)
	jaxrsPathRe     = regexp.MustCompile(`(?m)@Path\s*\(\s*["']([^"']+)["']\s*\)`)
	jaxrsMethodRe   = regexp.MustCompile(`(?m)@(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)`)
)

func extractRoutes(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Spring MVC mappings
	for _, m := range springMappingRe.FindAllStringSubmatchIndex(content, -1) {
		annotation := content[m[2]:m[3]]
		path := content[m[4]:m[5]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		method := strings.ToUpper(strings.TrimSuffix(annotation, "Mapping"))
		if method == "REQUEST" {
			method = "GET" // default for @RequestMapping
		}
		r, a := buildRouteRecords(method, path, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// JAX-RS: collect @Path locations, then match @GET/@POST etc to nearest preceding @Path
	type pathEntry struct {
		path string
		line int
	}
	var paths []pathEntry
	for _, m := range jaxrsPathRe.FindAllStringSubmatchIndex(content, -1) {
		p := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		paths = append(paths, pathEntry{path: p, line: line})
	}

	for _, m := range jaxrsMethodRe.FindAllStringSubmatchIndex(content, -1) {
		method := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		// Find nearest preceding @Path
		bestPath := ""
		for _, pe := range paths {
			if pe.line <= line {
				bestPath = pe.path
			}
		}
		if bestPath == "" {
			continue
		}
		r, a := buildRouteRecords(method, bestPath, line)
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
