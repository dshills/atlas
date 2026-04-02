package rustextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	reqwestRe     = regexp.MustCompile(`(?m)reqwest::(?:get|Client)`)
	hyperClientRe = regexp.MustCompile(`(?m)hyper::Client`)
	tonicRe       = regexp.MustCompile(`(?m)tonic::\w+`)
	tokioSpawnRe  = regexp.MustCompile(`(?m)tokio::spawn\s*\(`)
	threadSpawnRe = regexp.MustCompile(`(?m)(?:std::)?thread::spawn\s*\(`)
	rayonRe       = regexp.MustCompile(`(?m)\.par_iter\s*\(`)
)

func extractServices(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// HTTP clients → external_service + invokes_external_api
	type svcPattern struct {
		re      *regexp.Regexp
		svcType string
		source  string
	}
	httpPatterns := []svcPattern{
		{reqwestRe, "http_client", "reqwest"},
		{hyperClientRe, "http_client", "hyper::Client"},
		{tonicRe, "grpc_client", "tonic"},
	}

	for _, p := range httpPatterns {
		for _, m := range p.re.FindAllStringIndex(content, -1) {
			line := strings.Count(content[:m[0]], "\n") + 1
			if line-1 >= len(codeLines) || !codeLines[line-1] {
				continue
			}
			source := content[m[0]:m[1]]
			dataJSON, _ := json.Marshal(map[string]string{
				"type":   p.svcType,
				"source": source,
			})
			arts = append(arts, extractor.ArtifactRecord{
				ArtifactKind: "external_service",
				Name:         source,
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})
			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "invokes_external_api",
				Confidence:    "heuristic",
				Line:          line,
				RawTargetText: source,
			})
		}
	}

	// Background jobs → background_job artifact only
	type jobPattern struct {
		re      *regexp.Regexp
		jobType string
		name    string
	}
	jobPatterns := []jobPattern{
		{tokioSpawnRe, "async_task", "tokio::spawn"},
		{threadSpawnRe, "thread", "thread::spawn"},
		{rayonRe, "parallel", "par_iter"},
	}

	for _, p := range jobPatterns {
		for _, m := range p.re.FindAllStringIndex(content, -1) {
			line := strings.Count(content[:m[0]], "\n") + 1
			if line-1 >= len(codeLines) || !codeLines[line-1] {
				continue
			}
			dataJSON, _ := json.Marshal(map[string]string{
				"type": p.jobType,
				"name": p.name,
			})
			arts = append(arts, extractor.ArtifactRecord{
				ArtifactKind: "background_job",
				Name:         p.name,
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})
		}
	}

	return refs, arts
}
