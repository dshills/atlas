package csharpextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_HttpClient(t *testing.T) {
	content := `var client = new HttpClient();
var response = client.GetAsync("/api/users");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) < 1 {
		t.Fatalf("expected at least 1 service ref, got %d", len(refs))
	}

	foundHTTP := false
	for _, a := range arts {
		if a.ArtifactKind == "external_service" && a.Name == "HttpClient" {
			foundHTTP = true
		}
	}
	if !foundHTTP {
		t.Error("expected external_service artifact for HttpClient")
	}

	for _, r := range refs {
		if r.ReferenceKind != "invokes_external_api" {
			t.Errorf("expected reference kind 'invokes_external_api', got %q", r.ReferenceKind)
		}
		if r.Confidence != "heuristic" {
			t.Errorf("expected confidence 'heuristic', got %q", r.Confidence)
		}
	}
}

func TestExtractServices_BackgroundJobs(t *testing.T) {
	content := `Task.Run(() => ProcessItems());
var thread = new Thread(DoWork);
Parallel.ForEach(items, item => Process(item));
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractServices(content, lines, codeLines)

	bgCount := 0
	for _, a := range arts {
		if a.ArtifactKind == "background_job" {
			bgCount++
		}
	}
	if bgCount < 2 {
		t.Errorf("expected at least 2 background_job artifacts, got %d", bgCount)
	}

	if len(refs) < 2 {
		t.Errorf("expected at least 2 refs, got %d", len(refs))
	}
}

func TestExtractServices_WebRequest(t *testing.T) {
	content := `var req = WebRequest.Create("https://example.com/api");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 service artifact, got %d", len(arts))
	}
	if arts[0].Name != "WebRequest" {
		t.Errorf("expected 'WebRequest', got %q", arts[0].Name)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
}

func TestExtractServices_CommentedOut(t *testing.T) {
	content := `// var client = new HttpClient();
var x = 42;
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 0 {
		t.Errorf("expected 0 refs for commented service, got %d", len(refs))
	}
	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts for commented service, got %d", len(arts))
	}
}

func TestExtractServices_BackgroundService(t *testing.T) {
	content := `public class WorkerService : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken token) { }
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	_, arts := extractServices(content, lines, codeLines)

	foundBg := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "BackgroundService" {
			foundBg = true
		}
	}
	if !foundBg {
		t.Error("expected background_job artifact for BackgroundService")
	}
}
