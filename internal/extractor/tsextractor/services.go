package tsextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	fetchRe   = regexp.MustCompile("(?m)fetch\\s*\\(\\s*['\"`]([^'\"`]+)['\"`]")
	axiosRe   = regexp.MustCompile("(?m)axios\\.(get|post|put|delete|patch|request)\\s*\\(\\s*['\"`]([^'\"`]+)['\"`]")
	httpReqRe = regexp.MustCompile("(?m)(?:http|https)\\.(?:get|post|request)\\s*\\(\\s*['\"`]([^'\"`]+)['\"`]")
	workerRe  = regexp.MustCompile(`(?m)new\s+Worker\s*\(`)
	queueRe   = regexp.MustCompile(`(?m)(?:new\s+(?:Queue|Bull|Agenda)\s*\(|\.process\s*\()`)
)

func extractServices(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// fetch()
	for _, m := range fetchRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		url := content[m[2]:m[3]]
		r, a := buildServiceRecords("fetch", "fetch", url, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// axios.<method>()
	for _, m := range axiosRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		method := content[m[2]:m[3]]
		url := content[m[4]:m[5]]
		source := "axios." + method
		r, a := buildServiceRecords(source, source, url, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// http/https.get/post/request()
	for _, m := range httpReqRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		url := content[m[2]:m[3]]
		r, a := buildServiceRecords("http", "http", url, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// new Worker()
	for _, m := range workerRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"type": "worker",
			"name": "Worker",
		}
		dataJSON, _ := json.Marshal(dataMap)
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         "Worker",
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	// Queue/Bull/Agenda
	for _, m := range queueRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"type": "queue",
			"name": "Queue",
		}
		dataJSON, _ := json.Marshal(dataMap)
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         "Queue",
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	return refs, arts
}

func buildServiceRecords(name, source, url string, line int) (extractor.ReferenceRecord, extractor.ArtifactRecord) {
	dataMap := map[string]string{
		"type":   "http_client",
		"source": source,
	}
	dataJSON, _ := json.Marshal(dataMap)

	ref := extractor.ReferenceRecord{
		ReferenceKind: "invokes_external_api",
		Confidence:    "heuristic",
		Line:          line,
		RawTargetText: url,
	}

	art := extractor.ArtifactRecord{
		ArtifactKind: "external_service",
		Name:         name,
		DataJSON:     string(dataJSON),
		Confidence:   "heuristic",
	}

	return ref, art
}
