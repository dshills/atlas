package csharpextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractConfigAccess_EnvVar(t *testing.T) {
	content := `var db = Environment.GetEnvironmentVariable("DATABASE_URL");
var port = Environment.GetEnvironmentVariable("PORT");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

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
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}
	if arts[1].Name != "PORT" {
		t.Errorf("expected 'PORT', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_ConfigBracket(t *testing.T) {
	content := `var host = configuration["ConnectionStrings:Default"];
var key = config["AppSettings:ApiKey"];
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "ConnectionStrings:Default" {
		t.Errorf("expected 'ConnectionStrings:Default', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "config_key" {
		t.Errorf("expected artifact kind 'config_key', got %q", arts[0].ArtifactKind)
	}
	if refs[0].Confidence != "likely" {
		t.Errorf("expected confidence 'likely', got %q", refs[0].Confidence)
	}
	if arts[1].Name != "AppSettings:ApiKey" {
		t.Errorf("expected 'AppSettings:ApiKey', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_GetSection(t *testing.T) {
	content := `var section = configuration.GetSection("Logging");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref, got %d", len(refs))
	}
	if arts[0].Name != "Logging" {
		t.Errorf("expected 'Logging', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "config_key" {
		t.Errorf("expected artifact kind 'config_key', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractConfigAccess_GetValue(t *testing.T) {
	content := `var timeout = configuration.GetValue<int>("RequestTimeout");
var name = config.GetValue("AppName");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 config refs, got %d", len(refs))
	}
	if arts[0].Name != "RequestTimeout" {
		t.Errorf("expected 'RequestTimeout', got %q", arts[0].Name)
	}
	if arts[1].Name != "AppName" {
		t.Errorf("expected 'AppName', got %q", arts[1].Name)
	}
}

func TestExtractConfigAccess_CommentedOut(t *testing.T) {
	content := `// var x = Environment.GetEnvironmentVariable("COMMENTED");
var y = Environment.GetEnvironmentVariable("REAL_KEY");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractConfigAccess(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 config ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "REAL_KEY" {
		t.Errorf("expected 'REAL_KEY', got %q", arts[0].Name)
	}
}
