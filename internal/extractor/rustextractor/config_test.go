package rustextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess_EnvVar(t *testing.T) {
	content := `fn main() {
    let db = env::var("DATABASE_URL").unwrap();
    let port = env::var("PORT").unwrap_or("8080".to_string());
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

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

func TestExtractConfigAccess_StdEnvVar(t *testing.T) {
	content := `fn main() {
    let home = std::env::var("HOME").unwrap();
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref, got %d", len(refs))
	}
	if arts[0].Name != "HOME" {
		t.Errorf("expected 'HOME', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "env_var" {
		t.Errorf("expected artifact kind 'env_var', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractConfigAccess_DotenvVar(t *testing.T) {
	content := `fn main() {
    let key = dotenvy::var("API_KEY").unwrap();
    let secret = dotenv::var("SECRET").unwrap();
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "API_KEY" {
		t.Errorf("expected 'API_KEY', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "env_var" {
		t.Errorf("expected artifact kind 'env_var', got %q", arts[0].ArtifactKind)
	}
	if arts[1].Name != "SECRET" {
		t.Errorf("expected 'SECRET', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_ConfigGet(t *testing.T) {
	content := `fn main() {
    let host = config.get_string("database.url").unwrap();
    let port = config.get::<i32>("server.port").unwrap();
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "database.url" {
		t.Errorf("expected 'database.url', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "config_key" {
		t.Errorf("expected artifact kind 'config_key', got %q", arts[0].ArtifactKind)
	}
	if arts[1].Name != "server.port" {
		t.Errorf("expected 'server.port', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `fn main() {
    // env::var("COMMENTED")
    let real = env::var("REAL_KEY").unwrap();
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "REAL_KEY" {
		t.Errorf("expected 'REAL_KEY', got %q", arts[0].Name)
	}
}
