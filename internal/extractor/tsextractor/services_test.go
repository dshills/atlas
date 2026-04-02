package tsextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_Fetch(t *testing.T) {
	content := `const data = await fetch('https://api.example.com/users')
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "external_service" {
		t.Errorf("expected artifact kind 'external_service', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "fetch" {
		t.Errorf("expected name 'fetch', got %q", arts[0].Name)
	}
	if arts[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", arts[0].Confidence)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "invokes_external_api" {
		t.Errorf("expected reference kind 'invokes_external_api', got %q", refs[0].ReferenceKind)
	}
	if refs[0].RawTargetText != "https://api.example.com/users" {
		t.Errorf("expected raw target 'https://api.example.com/users', got %q", refs[0].RawTargetText)
	}
}

func TestExtractServices_Axios(t *testing.T) {
	content := `const resp = await axios.get('/api/data')
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "external_service" {
		t.Errorf("expected artifact kind 'external_service', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "axios.get" {
		t.Errorf("expected name 'axios.get', got %q", arts[0].Name)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].RawTargetText != "/api/data" {
		t.Errorf("expected raw target '/api/data', got %q", refs[0].RawTargetText)
	}
}

func TestExtractServices_Worker(t *testing.T) {
	content := `const w = new Worker('worker.js')
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "background_job" {
		t.Errorf("expected artifact kind 'background_job', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "Worker" {
		t.Errorf("expected name 'Worker', got %q", arts[0].Name)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for Worker, got %d", len(refs))
	}
}

func TestExtractServices_CommentedFetch(t *testing.T) {
	content := `// const data = await fetch('https://api.example.com/users')
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts for commented fetch, got %d", len(arts))
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for commented fetch, got %d", len(refs))
	}
}

func TestExtractServices_Queue(t *testing.T) {
	content := `const queue = new Queue('email-queue')
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	_, arts := extractServices(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "background_job" {
		t.Errorf("expected artifact kind 'background_job', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "Queue" {
		t.Errorf("expected name 'Queue', got %q", arts[0].Name)
	}
}
