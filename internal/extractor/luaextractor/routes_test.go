package luaextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractRoutes_Lapis(t *testing.T) {
	content := `local lapis = require("lapis")
local app = lapis.Application()

app:get("/users", function(self)
    return "users"
end)

app:post("/users", function(self)
    return "created"
end)
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}

	if arts[0].Name != "GET /users" {
		t.Errorf("expected 'GET /users', got %q", arts[0].Name)
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

	if arts[1].Name != "POST /users" {
		t.Errorf("expected 'POST /users', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_SelfMethod(t *testing.T) {
	content := `self:get("/api/items", function(self)
    return "items"
end)

self:delete("/api/items", function(self)
    return "deleted"
end)
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if arts[0].Name != "GET /api/items" {
		t.Errorf("expected 'GET /api/items', got %q", arts[0].Name)
	}
	if arts[1].Name != "DELETE /api/items" {
		t.Errorf("expected 'DELETE /api/items', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `-- app:get("/hidden", function() end)
app:get("/visible", function(self)
    return "ok"
end)
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented route excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET /visible" {
		t.Errorf("expected 'GET /visible', got %q", arts[0].Name)
	}
}
