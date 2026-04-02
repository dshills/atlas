package javaextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	httpClientRe   = regexp.MustCompile(`(?m)(?:HttpClient|RestTemplate|WebClient|OkHttpClient)\s*[\w.]*\s*=`)
	restTemplateRe = regexp.MustCompile(`(?m)restTemplate\.(getForObject|postForObject|exchange|getForEntity|postForEntity)\s*\(\s*"([^"]+)"`)
	webClientRe    = regexp.MustCompile(`(?m)\.uri\s*\(\s*"([^"]+)"`)
	executorRe     = regexp.MustCompile(`(?m)(?:ExecutorService|CompletableFuture|ScheduledExecutorService)\s`)
	asyncRe        = regexp.MustCompile(`(?m)@Async`)
	newThreadRe    = regexp.MustCompile(`(?m)new\s+Thread\s*\(`)
)

func extractServices(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// RestTemplate with URL
	for _, m := range restTemplateRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		method := content[m[2]:m[3]]
		url := content[m[4]:m[5]]
		source := "RestTemplate." + method
		r, a := buildServiceRecords(source, source, url, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// WebClient .uri("...")
	for _, m := range webClientRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		url := content[m[2]:m[3]]
		r, a := buildServiceRecords("WebClient", "WebClient", url, line)
		refs = append(refs, r)
		arts = append(arts, a)
	}

	// Generic HTTP client declarations (without URL)
	for _, m := range httpClientRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		matched := content[m[0]:m[1]]
		name := "HttpClient"
		if strings.Contains(matched, "RestTemplate") {
			name = "RestTemplate"
		} else if strings.Contains(matched, "WebClient") {
			name = "WebClient"
		} else if strings.Contains(matched, "OkHttpClient") {
			name = "OkHttpClient"
		}
		dataMap := map[string]string{
			"type":   "http_client",
			"source": name,
		}
		dataJSON, _ := json.Marshal(dataMap)
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "external_service",
			Name:         name,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	// ExecutorService / CompletableFuture / ScheduledExecutorService
	for _, m := range executorRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		matched := strings.TrimSpace(content[m[0]:m[1]])
		dataMap := map[string]string{
			"type": "executor",
			"name": matched,
		}
		dataJSON, _ := json.Marshal(dataMap)
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         matched,
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	// @Async
	for _, m := range asyncRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"type": "async",
			"name": "@Async",
		}
		dataJSON, _ := json.Marshal(dataMap)
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         "@Async",
			DataJSON:     string(dataJSON),
			Confidence:   "heuristic",
		})
	}

	// new Thread(
	for _, m := range newThreadRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		dataMap := map[string]string{
			"type": "thread",
			"name": "Thread",
		}
		dataJSON, _ := json.Marshal(dataMap)
		arts = append(arts, extractor.ArtifactRecord{
			ArtifactKind: "background_job",
			Name:         "Thread",
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
