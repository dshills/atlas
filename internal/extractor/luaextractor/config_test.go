package luaextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess(t *testing.T) {
	content := `local db_host = os.getenv("DB_HOST")
local port = os.getenv('PORT')
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 config artifacts, got %d", len(arts))
	}

	if refs[0].ReferenceKind != "uses_config" {
		t.Errorf("expected reference kind 'uses_config', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}
	if refs[0].RawTargetText != "DB_HOST" {
		t.Errorf("expected raw target 'DB_HOST', got %q", refs[0].RawTargetText)
	}

	if arts[0].ArtifactKind != "env_var" {
		t.Errorf("expected artifact kind 'env_var', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "DB_HOST" {
		t.Errorf("expected name 'DB_HOST', got %q", arts[0].Name)
	}

	if refs[1].RawTargetText != "PORT" {
		t.Errorf("expected raw target 'PORT', got %q", refs[1].RawTargetText)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `-- local secret = os.getenv("SECRET")
local visible = os.getenv("VISIBLE")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, _ := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if refs[0].RawTargetText != "VISIBLE" {
		t.Errorf("expected 'VISIBLE', got %q", refs[0].RawTargetText)
	}
}
