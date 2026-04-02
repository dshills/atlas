package javaextractor

import (
	"encoding/json"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractRoutes_SpringMVC(t *testing.T) {
	content := `@RestController
public class UserController {
    @GetMapping("/users")
    public List<User> list() {}

    @PostMapping("/users")
    public User create(@RequestBody User user) {}

    @DeleteMapping("/users/{id}")
    public void delete(@PathVariable Long id) {}
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 3 {
		t.Fatalf("expected 3 route refs, got %d", len(refs))
	}
	if len(arts) != 3 {
		t.Fatalf("expected 3 route artifacts, got %d", len(arts))
	}

	if arts[0].Name != "GET /users" {
		t.Errorf("expected 'GET /users', got %q", arts[0].Name)
	}
	if arts[1].Name != "POST /users" {
		t.Errorf("expected 'POST /users', got %q", arts[1].Name)
	}
	if arts[2].Name != "DELETE /users/{id}" {
		t.Errorf("expected 'DELETE /users/{id}', got %q", arts[2].Name)
	}
	if refs[0].ReferenceKind != "registers_route" {
		t.Errorf("expected reference kind 'registers_route', got %q", refs[0].ReferenceKind)
	}
	if refs[0].RawTargetText != "/users" {
		t.Errorf("expected raw target '/users', got %q", refs[0].RawTargetText)
	}
}

func TestExtractRoutes_SpringRequestMapping(t *testing.T) {
	content := `@RequestMapping("/api/health")
public String health() { return "ok"; }
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 route artifact, got %d", len(arts))
	}
	if arts[0].Name != "GET /api/health" {
		t.Errorf("expected 'GET /api/health', got %q", arts[0].Name)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref, got %d", len(refs))
	}
}

func TestExtractRoutes_JAXRS(t *testing.T) {
	content := `@Path("/users")
public class UserResource {
    @GET
    public List<User> list() {}

    @POST
    public User create(User user) {}
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

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
	if arts[1].Name != "POST /users" {
		t.Errorf("expected 'POST /users', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `// @GetMapping("/commented-out")
@GetMapping("/real-route")
public String handler() {}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented route excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET /real-route" {
		t.Errorf("expected 'GET /real-route', got %q", arts[0].Name)
	}
}

func TestExtractRoutes_DataJSON(t *testing.T) {
	content := `@GetMapping("/api/items")
public List<Item> items() {}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "java")

	_, arts := extractRoutes(content, lines, codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(arts[0].DataJSON), &data); err != nil {
		t.Fatalf("failed to unmarshal DataJSON: %v", err)
	}
	if data["method"] != "GET" {
		t.Errorf("expected method 'GET', got %q", data["method"])
	}
	if data["path"] != "/api/items" {
		t.Errorf("expected path '/api/items', got %q", data["path"])
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
