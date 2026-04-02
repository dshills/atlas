package pyextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_RequestsGet(t *testing.T) {
	content := `import requests
response = requests.get('https://api.example.com')
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 external_service artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "external_service" {
		t.Errorf("expected artifact kind 'external_service', got %q", arts[0].ArtifactKind)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "invokes_external_api" {
		t.Errorf("expected reference kind 'invokes_external_api', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
}

func TestExtractServices_SharedTask(t *testing.T) {
	content := `from celery import shared_task

@shared_task
def send_email(to, subject, body):
    pass
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 0 {
		t.Fatalf("expected 0 refs for background_job, got %d", len(refs))
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 background_job artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "background_job" {
		t.Errorf("expected artifact kind 'background_job', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractServices_AsyncioCreateTask(t *testing.T) {
	content := `import asyncio
task = asyncio.create_task(coro())
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 0 {
		t.Fatalf("expected 0 refs for background_job, got %d", len(refs))
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 background_job artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "background_job" {
		t.Errorf("expected artifact kind 'background_job', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "create_task" {
		t.Errorf("expected name 'create_task', got %q", arts[0].Name)
	}
}

func TestExtractServices_CommentedOut(t *testing.T) {
	content := `# requests.get('https://api.example.com')
real = "hello world"
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 0 {
		t.Fatalf("expected 0 refs for commented code, got %d", len(refs))
	}
	if len(arts) != 0 {
		t.Fatalf("expected 0 artifacts for commented code, got %d", len(arts))
	}
}
