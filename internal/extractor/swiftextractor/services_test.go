package swiftextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_URLSession(t *testing.T) {
	content := `import Foundation

func fetchData() {
    let task = URLSession.shared.dataTask(with: url) { data, response, error in
        print(data)
    }
    task.resume()
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractServices(content, lines, codeLines)

	hasExtService := false
	for _, a := range arts {
		if a.ArtifactKind == "external_service" && a.Name == "URLSession" {
			hasExtService = true
			break
		}
	}
	if !hasExtService {
		t.Error("expected external_service artifact for URLSession")
	}

	hasRef := false
	for _, r := range refs {
		if r.ReferenceKind == "invokes_external_api" && r.RawTargetText == "URLSession" {
			hasRef = true
			break
		}
	}
	if !hasRef {
		t.Error("expected invokes_external_api reference for URLSession")
	}
}

func TestExtractServices_Alamofire(t *testing.T) {
	content := `import Alamofire

func fetch() {
    Alamofire.request("https://api.example.com").response { resp in
        print(resp)
    }
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractServices(content, lines, codeLines)

	hasExtService := false
	for _, a := range arts {
		if a.ArtifactKind == "external_service" && a.Name == "Alamofire" {
			hasExtService = true
			break
		}
	}
	if !hasExtService {
		t.Error("expected external_service artifact for Alamofire")
	}

	hasRef := false
	for _, r := range refs {
		if r.ReferenceKind == "invokes_external_api" && r.RawTargetText == "Alamofire" {
			hasRef = true
			break
		}
	}
	if !hasRef {
		t.Error("expected invokes_external_api reference for Alamofire")
	}
}

func TestExtractServices_DispatchQueue(t *testing.T) {
	content := `func process() {
    DispatchQueue.global().async {
        heavyComputation()
    }
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "swift")

	_, arts := extractServices(content, lines, codeLines)

	hasJob := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "DispatchQueue" {
			hasJob = true
			break
		}
	}
	if !hasJob {
		t.Error("expected background_job artifact for DispatchQueue")
	}
}

func TestExtractServices_Task(t *testing.T) {
	content := `func doWork() {
    Task {
        await performAsync()
    }
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "swift")

	_, arts := extractServices(content, lines, codeLines)

	hasJob := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "Task" {
			hasJob = true
			break
		}
	}
	if !hasJob {
		t.Error("expected background_job artifact for Task")
	}
}

func TestExtractServices_CommentedOut(t *testing.T) {
	content := `// URLSession.shared.dataTask(with: url)
let session = URLSession.shared
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractServices(content, lines, codeLines)

	extServiceCount := 0
	for _, a := range arts {
		if a.ArtifactKind == "external_service" {
			extServiceCount++
		}
	}
	if extServiceCount != 1 {
		t.Fatalf("expected 1 external_service artifact (commented excluded), got %d", extServiceCount)
	}

	apiRefCount := 0
	for _, r := range refs {
		if r.ReferenceKind == "invokes_external_api" {
			apiRefCount++
		}
	}
	if apiRefCount != 1 {
		t.Fatalf("expected 1 invokes_external_api ref (commented excluded), got %d", apiRefCount)
	}
}
