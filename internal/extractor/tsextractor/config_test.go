package tsextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess_ProcessEnvDot(t *testing.T) {
	content := `const db = process.env.DATABASE_URL
const port = process.env.PORT
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

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
	if arts[1].Name != "PORT" {
		t.Errorf("expected 'PORT', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_ProcessEnvBracket(t *testing.T) {
	content := `const key = process.env['API_KEY']
const secret = process.env["SECRET_TOKEN"]
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "API_KEY" {
		t.Errorf("expected 'API_KEY', got %q", arts[0].Name)
	}
	if arts[1].Name != "SECRET_TOKEN" {
		t.Errorf("expected 'SECRET_TOKEN', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_ConfigGet(t *testing.T) {
	content := `const host = config.get('database.host')
const port = config.get<number>('database.port')
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "typescript")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "database.host" {
		t.Errorf("expected 'database.host', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "config_key" {
		t.Errorf("expected artifact kind 'config_key', got %q", arts[0].ArtifactKind)
	}
	if arts[1].Name != "database.port" {
		t.Errorf("expected 'database.port', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `// process.env.COMMENTED
process.env.REAL_KEY
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "REAL_KEY" {
		t.Errorf("expected 'REAL_KEY', got %q", arts[0].Name)
	}
}
