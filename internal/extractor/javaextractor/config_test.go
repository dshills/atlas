package javaextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess_SystemGetenv(t *testing.T) {
	content := `String dbUrl = System.getenv("DATABASE_URL");
String port = System.getenv("PORT");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

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

func TestExtractConfigAccess_SystemGetProperty(t *testing.T) {
	content := `String home = System.getProperty("user.home");
String javaVersion = System.getProperty("java.version");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "user.home" {
		t.Errorf("expected 'user.home', got %q", arts[0].Name)
	}
	if arts[1].Name != "java.version" {
		t.Errorf("expected 'java.version', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_SpringValue(t *testing.T) {
	content := `@Value("${server.port}")
private int port;

@Value("${app.name}")
private String appName;
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 config artifacts, got %d", len(arts))
	}

	if arts[0].Name != "server.port" {
		t.Errorf("expected 'server.port', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "config_key" {
		t.Errorf("expected artifact kind 'config_key', got %q", arts[0].ArtifactKind)
	}
	if arts[1].Name != "app.name" {
		t.Errorf("expected 'app.name', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `// String dbUrl = System.getenv("COMMENTED");
String port = System.getenv("REAL_KEY");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "REAL_KEY" {
		t.Errorf("expected 'REAL_KEY', got %q", arts[0].Name)
	}
}
