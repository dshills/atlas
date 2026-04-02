package javaextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_RestTemplate(t *testing.T) {
	content := `String result = restTemplate.getForObject("https://api.example.com/users", String.class);
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractServices(content, lines, codeLines)

	foundService := false
	for _, a := range arts {
		if a.ArtifactKind == "external_service" && a.Name == "RestTemplate.getForObject" {
			foundService = true
		}
	}
	if !foundService {
		t.Errorf("expected external_service artifact for RestTemplate, got %v", arts)
	}

	foundRef := false
	for _, r := range refs {
		if r.ReferenceKind == "invokes_external_api" && r.RawTargetText == "https://api.example.com/users" {
			foundRef = true
		}
	}
	if !foundRef {
		t.Errorf("expected invokes_external_api ref, got %v", refs)
	}
}

func TestExtractServices_HttpClientDeclaration(t *testing.T) {
	content := `HttpClient client = HttpClient.newHttpClient();
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	_, arts := extractServices(content, lines, codeLines)

	found := false
	for _, a := range arts {
		if a.ArtifactKind == "external_service" && a.Name == "HttpClient" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected external_service artifact for HttpClient, got %v", arts)
	}
}

func TestExtractServices_ExecutorService(t *testing.T) {
	content := `ExecutorService executor = Executors.newFixedThreadPool(4);
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	_, arts := extractServices(content, lines, codeLines)

	found := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "ExecutorService" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected background_job artifact for ExecutorService, got %v", arts)
	}
}

func TestExtractServices_CompletableFuture(t *testing.T) {
	content := `CompletableFuture future = CompletableFuture.supplyAsync(() -> compute());
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	_, arts := extractServices(content, lines, codeLines)

	found := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "CompletableFuture" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected background_job artifact for CompletableFuture, got %v", arts)
	}
}

func TestExtractServices_AsyncAnnotation(t *testing.T) {
	content := `@Async
public void sendEmail(String to) {}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	_, arts := extractServices(content, lines, codeLines)

	found := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "@Async" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected background_job artifact for @Async, got %v", arts)
	}
}

func TestExtractServices_NewThread(t *testing.T) {
	content := `Thread t = new Thread(() -> process());
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	_, arts := extractServices(content, lines, codeLines)

	found := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "Thread" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected background_job artifact for Thread, got %v", arts)
	}
}

func TestExtractServices_CommentedOut(t *testing.T) {
	content := `// RestTemplate restTemplate = new RestTemplate();
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts for commented service, got %d", len(arts))
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for commented service, got %d", len(refs))
	}
}
