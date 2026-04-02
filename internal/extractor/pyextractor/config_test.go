package pyextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess_OsEnvironBracket(t *testing.T) {
	content := `import os
db_url = os.environ['DATABASE_URL']
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref, got %d", len(refs))
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 config artifact, got %d", len(arts))
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
}

func TestExtractConfigAccess_OsEnvironGet(t *testing.T) {
	content := `import os
api_key = os.environ.get('API_KEY')
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref, got %d", len(refs))
	}
	if arts[0].Name != "API_KEY" {
		t.Errorf("expected 'API_KEY', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "env_var" {
		t.Errorf("expected artifact kind 'env_var', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractConfigAccess_OsGetenv(t *testing.T) {
	content := `import os
secret = os.getenv('SECRET_KEY')
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref, got %d", len(refs))
	}
	if arts[0].Name != "SECRET_KEY" {
		t.Errorf("expected 'SECRET_KEY', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "env_var" {
		t.Errorf("expected artifact kind 'env_var', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractConfigAccess_SettingsDot(t *testing.T) {
	content := `from django.conf import settings
debug = settings.DEBUG
db_name = settings.DATABASE_NAME
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "DEBUG" {
		t.Errorf("expected 'DEBUG', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "config_key" {
		t.Errorf("expected artifact kind 'config_key', got %q", arts[0].ArtifactKind)
	}
	if arts[1].Name != "DATABASE_NAME" {
		t.Errorf("expected 'DATABASE_NAME', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `# os.getenv('COMMENTED')
secret = os.getenv('REAL_KEY')
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "REAL_KEY" {
		t.Errorf("expected 'REAL_KEY', got %q", arts[0].Name)
	}
}
