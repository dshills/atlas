package rustextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	// Actix/Rocket attribute macros: #[get("/users")]
	attrRouteRe = regexp.MustCompile(`#\[(get|post|put|delete|patch|head|options)\s*\(\s*"([^"]+)"`)
	// Axum builder: .route("/users", get(list_users))
	axumRouteRe = regexp.MustCompile(`\.route\s*\(\s*"([^"]+)"\s*,\s*(get|post|put|delete|patch|head|options)\s*\(`)
	// Actix resource: .route("/items", web::post().to(create_item))
	actixRouteRe = regexp.MustCompile(`\.route\s*\(\s*"([^"]+)"\s*,\s*web::(get|post|put|delete|patch|head|options)\s*\(\s*\)\s*\.to\s*\(\s*(\w+)`)
)

func extractRoutes(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// Actix/Rocket attribute macros
	for _, m := range attrRouteRe.FindAllStringSubmatchIndex(content, -1) {
		method := strings.ToUpper(content[m[2]:m[3]])
		path := content[m[4]:m[5]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRustRouteRecords(method, path, "", "exact", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// Axum builder routes
	for _, m := range axumRouteRe.FindAllStringSubmatchIndex(content, -1) {
		path := content[m[2]:m[3]]
		method := strings.ToUpper(content[m[4]:m[5]])
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRustRouteRecords(method, path, "", "heuristic", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// Actix resource routes
	for _, m := range actixRouteRe.FindAllStringSubmatchIndex(content, -1) {
		path := content[m[2]:m[3]]
		method := strings.ToUpper(content[m[4]:m[5]])
		handler := content[m[6]:m[7]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		r, a := buildRustRouteRecords(method, path, handler, "heuristic", line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	return refs, arts
}

func buildRustRouteRecords(method, path, handler, confidence string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	name := method + " " + path

	dataMap := map[string]string{
		"method":  method,
		"path":    path,
		"handler": handler,
	}
	dataJSON, _ := json.Marshal(dataMap)

	ref := extractor.ReferenceRecord{
		ReferenceKind: "registers_route",
		Confidence:    confidence,
		Line:          line,
		RawTargetText: path,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "route",
		Name:         name,
		DataJSON:     string(dataJSON),
		Confidence:   confidence,
	}

	return ref, art
}
