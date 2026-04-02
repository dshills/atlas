package swiftextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractRoutes_VaporGet(t *testing.T) {
	content := `import Vapor

func routes(_ app: Application) throws {
    app.get("users") { req in
        return "users list"
    }
    app.post("users") { req in
        return "created"
    }
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}

	if arts[0].Name != "GET users" {
		t.Errorf("expected 'GET users', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "route" {
		t.Errorf("expected artifact kind 'route', got %q", arts[0].ArtifactKind)
	}
	if refs[0].ReferenceKind != "registers_route" {
		t.Errorf("expected reference kind 'registers_route', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "likely" {
		t.Errorf("expected confidence 'likely', got %q", refs[0].Confidence)
	}

	if arts[1].Name != "POST users" {
		t.Errorf("expected 'POST users', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_VaporRouter(t *testing.T) {
	content := `let router = app.router
router.get("api/items") { req in
    return items
}
router.delete("api/items") { req in
    return "deleted"
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if arts[0].Name != "GET api/items" {
		t.Errorf("expected 'GET api/items', got %q", arts[0].Name)
	}
	if arts[1].Name != "DELETE api/items" {
		t.Errorf("expected 'DELETE api/items', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `// app.get("hidden") { req in return "nope" }
app.get("visible") { req in
    return "ok"
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented route excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET visible" {
		t.Errorf("expected 'GET visible', got %q", arts[0].Name)
	}
}

func TestExtractRoutes_Empty(t *testing.T) {
	content := `import Vapor
let x = 42
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 0 {
		t.Errorf("expected 0 route refs, got %d", len(refs))
	}
	if len(arts) != 0 {
		t.Errorf("expected 0 route artifacts, got %d", len(arts))
	}
}

func splitLines(content string) []string {
	var lines []string
	start := 0
	for i, c := range content {
		if c == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}
