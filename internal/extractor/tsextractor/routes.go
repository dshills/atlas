package tsextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	expressRouteRe    = regexp.MustCompile("(?m)(?:app|router|server)\\.(get|post|put|delete|patch|head|options|all|use)\\s*\\(\\s*['\"`]([^'\"`]+)['\"`]")
	nestDecoratorRe   = regexp.MustCompile("(?m)@(Get|Post|Put|Delete|Patch|Head|Options|All)\\s*\\(\\s*['\"`]([^'\"`]*?)['\"`]\\s*\\)")
	nextRouteExportRe = regexp.MustCompile(`(?m)^export\s+(?:async\s+)?function\s+(GET|POST|PUT|DELETE|PATCH)\s*\(`)
)

func extractRoutes(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Express-style routes
	for _, m := range expressRouteRe.FindAllStringSubmatchIndex(content, -1) {
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

	// NestJS decorator routes
	for _, m := range nestDecoratorRe.FindAllStringSubmatchIndex(content, -1) {
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

	// Next.js route exports
	for _, m := range nextRouteExportRe.FindAllStringSubmatchIndex(content, -1) {
		method := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRouteRecords(method, "", line)
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
