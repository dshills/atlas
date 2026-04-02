package swiftextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess_ProcessInfo(t *testing.T) {
	content := `func configure() {
    let db = ProcessInfo.processInfo.environment["DATABASE_URL"]
    let port = ProcessInfo.processInfo.environment["PORT"]
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 config artifacts, got %d", len(arts))
	}

	if arts[0].Name != "DATABASE_URL" {
		t.Errorf("expected 'DATABASE_URL', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "env_var" {
		t.Errorf("expected artifact kind 'env_var', got %q", arts[0].ArtifactKind)
	}
	if refs[0].ReferenceKind != "uses_config" {
		t.Errorf("expected reference kind 'uses_config', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}
	if arts[1].Name != "PORT" {
		t.Errorf("expected 'PORT', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_EnvironmentGet(t *testing.T) {
	content := `func configure(_ app: Application) {
    let key = Environment.get("API_KEY")
    let secret = Environment.get("SECRET")
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "API_KEY" {
		t.Errorf("expected 'API_KEY', got %q", arts[0].Name)
	}
	if arts[1].Name != "SECRET" {
		t.Errorf("expected 'SECRET', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `func configure() {
    // ProcessInfo.processInfo.environment["COMMENTED"]
    let real = ProcessInfo.processInfo.environment["REAL_KEY"]
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "REAL_KEY" {
		t.Errorf("expected 'REAL_KEY', got %q", arts[0].Name)
	}
}

func TestExtractConfigAccess_Empty(t *testing.T) {
	content := `let x = 42
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 0 {
		t.Errorf("expected 0 config refs, got %d", len(refs))
	}
	if len(arts) != 0 {
		t.Errorf("expected 0 config artifacts, got %d", len(arts))
	}
}
