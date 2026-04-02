package csharpextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

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

func TestExtractRoutes_ASPNetAttributes(t *testing.T) {
	content := `[ApiController]
[Route("api/[controller]")]
public class UsersController : ControllerBase
{
    [HttpGet("users/{id}")]
    public IActionResult GetUser(int id) { }

    [HttpPost("users")]
    public IActionResult CreateUser() { }

    [HttpDelete("users/{id}")]
    public IActionResult DeleteUser(int id) { }
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractRoutes(content, lines, codeLines)

	// 3 HttpX attributes + 1 Route attribute = 4
	if len(arts) != 4 {
		t.Fatalf("expected 4 route artifacts, got %d", len(arts))
	}

	// HttpX attributes are processed first, then Route attributes
	// Check HttpGet
	if arts[0].Name != "GET users/{id}" {
		t.Errorf("expected 'GET users/{id}', got %q", arts[0].Name)
	}

	// Check HttpPost
	if arts[1].Name != "POST users" {
		t.Errorf("expected 'POST users', got %q", arts[1].Name)
	}

	// Check HttpDelete
	if arts[2].Name != "DELETE users/{id}" {
		t.Errorf("expected 'DELETE users/{id}', got %q", arts[2].Name)
	}

	// Check Route attribute
	if arts[3].Name != "ANY api/[controller]" {
		t.Errorf("expected 'ANY api/[controller]', got %q", arts[3].Name)
	}

	for _, r := range refs {
		if r.ReferenceKind != "registers_route" {
			t.Errorf("expected reference kind 'registers_route', got %q", r.ReferenceKind)
		}
		if r.Confidence != "likely" {
			t.Errorf("expected confidence 'likely', got %q", r.Confidence)
		}
	}
}

func TestExtractRoutes_MinimalAPI(t *testing.T) {
	content := `var app = builder.Build();

app.MapGet("/api/users", () => Results.Ok());
app.MapPost("/api/users", () => Results.Created());
app.MapDelete("/api/users/{id}", (int id) => Results.Ok());
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(arts) != 3 {
		t.Fatalf("expected 3 route artifacts, got %d", len(arts))
	}

	expected := []string{"GET /api/users", "POST /api/users", "DELETE /api/users/{id}"}
	for i, exp := range expected {
		if arts[i].Name != exp {
			t.Errorf("route %d: expected %q, got %q", i, exp, arts[i].Name)
		}
	}

	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `// [HttpGet("commented")]
[HttpGet("real")]
public IActionResult Get() { }
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET real" {
		t.Errorf("expected 'GET real', got %q", arts[0].Name)
	}
}

func TestExtractRoutes_HttpPut(t *testing.T) {
	content := `[HttpPut("items/{id}")]
public IActionResult UpdateItem(int id) { }

[HttpPatch("items/{id}")]
public IActionResult PatchItem(int id) { }
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	_, arts := extractRoutes(content, lines, codeLines)

	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}
	if arts[0].Name != "PUT items/{id}" {
		t.Errorf("expected 'PUT items/{id}', got %q", arts[0].Name)
	}
	if arts[1].Name != "PATCH items/{id}" {
		t.Errorf("expected 'PATCH items/{id}', got %q", arts[1].Name)
	}
}
