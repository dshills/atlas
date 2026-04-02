package swiftextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	swiftHTTPClientRe = regexp.MustCompile(`(?m)\b(URLSession|Alamofire)\b`)
	swiftBgJobRe      = regexp.MustCompile(`(?m)\b(DispatchQueue|async\s+let)\b`)
	swiftTaskRe       = regexp.MustCompile(`(?m)\bTask\s*\{`)
)

func extractServices(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// HTTP clients: URLSession, Alamofire
	for _, m := range swiftHTTPClientRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataJSON, _ := json.Marshal(map[string]string{"name": name})
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

	// Background jobs: DispatchQueue, async let
	for _, m := range swiftBgJobRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataJSON, _ := json.Marshal(map[string]string{"name": name})
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

	// Task { ... }
	for _, m := range swiftTaskRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataJSON, _ := json.Marshal(map[string]string{"name": "Task"})
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "invokes_external_api",
			Confidence:    "heuristic",
			Line:          line,
			RawTargetText: "Task",
		})
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         "Task",
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	return refs, arts
}
