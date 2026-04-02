package pyextractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	requestsRe   = regexp.MustCompile(`(?m)requests\.(get|post|put|delete|patch|head|options)\s*\(`)
	httpxRe      = regexp.MustCompile(`(?m)httpx\.(get|post|put|delete|patch|head|options)\s*\(`)
	urllibRe     = regexp.MustCompile(`(?m)urllib\.request\.urlopen\s*\(`)
	celeryTaskRe = regexp.MustCompile(`(?m)@(?:app\.task|shared_task|celery\.task)`)
	threadRe     = regexp.MustCompile(`(?m)threading\.Thread\s*\(`)
	asyncTaskRe  = regexp.MustCompile(`(?m)asyncio\.create_task\s*\(`)
	subprocessRe = regexp.MustCompile(`(?m)subprocess\.(?:run|Popen|call)\s*\(`)
)

func extractServices(content string, _ []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord) {
	var refs []extractor.ReferenceRecord
	var arts []extractor.ArtifactRecord

	// HTTP client patterns: requests, httpx, urllib
	type httpPattern struct {
		re        *regexp.Regexp
		source    string
		hasMethod bool
	}
	httpPatterns := []httpPattern{
		{requestsRe, "requests", true},
		{httpxRe, "httpx", true},
		{urllibRe, "urllib", false},
	}

	for _, hp := range httpPatterns {
		for _, m := range hp.re.FindAllStringSubmatchIndex(content, -1) {
			line := strings.Count(content[:m[0]], "\n") + 1
			if line-1 >= len(codeLines) || !codeLines[line-1] {
				continue
			}

			var sourceName string
			if hp.hasMethod {
				method := content[m[2]:m[3]]
				sourceName = hp.source + "." + method
			} else {
				sourceName = hp.source + ".urlopen"
			}

			dataMap := map[string]string{
				"type":   "http_client",
				"source": sourceName,
			}
			dataJSON, _ := json.Marshal(dataMap)

			refs = append(refs, extractor.ReferenceRecord{
				ReferenceKind: "invokes_external_api",
				Confidence:    "heuristic",
				Line:          line,
				RawTargetText: sourceName,
			})
			arts = append(arts, extractor.ArtifactRecord{
				ArtifactKind: "external_service",
				Name:         sourceName,
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})
		}
	}

	// Background job patterns
	type bgPattern struct {
		re      *regexp.Regexp
		jobType string
		jobName string
	}
	bgPatterns := []bgPattern{
		{celeryTaskRe, "celery_task", "app.task"},
		{threadRe, "thread", "Thread"},
		{asyncTaskRe, "async_task", "create_task"},
		{subprocessRe, "subprocess", "subprocess"},
	}

	for _, bp := range bgPatterns {
		for _, m := range bp.re.FindAllStringIndex(content, -1) {
			line := strings.Count(content[:m[0]], "\n") + 1
			if line-1 >= len(codeLines) || !codeLines[line-1] {
				continue
			}

			dataMap := map[string]string{
				"type": bp.jobType,
				"name": bp.jobName,
			}
			dataJSON, _ := json.Marshal(dataMap)

			arts = append(arts, extractor.ArtifactRecord{
				ArtifactKind: "background_job",
				Name:         bp.jobName,
				DataJSON:     string(dataJSON),
				Confidence:   "heuristic",
			})
		}
	}

	return refs, arts
}
