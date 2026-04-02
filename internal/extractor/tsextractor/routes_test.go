package tsextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractRoutes_Express(t *testing.T) {
	content := `const express = require('express')
const app = express()

app.get('/users', getUsers)
router.post('/api/items', createItem)
server.delete('/api/items/:id', deleteItem)
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 3 {
		t.Fatalf("expected 3 route refs, got %d", len(refs))
	}
	if len(arts) != 3 {
		t.Fatalf("expected 3 route artifacts, got %d", len(arts))
	}

	// Check first route
	if arts[0].Name != "GET /users" {
		t.Errorf("expected 'GET /users', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "route" {
		t.Errorf("expected artifact kind 'route', got %q", arts[0].ArtifactKind)
	}
	if refs[0].ReferenceKind != "registers_route" {
		t.Errorf("expected reference kind 'registers_route', got %q", refs[0].ReferenceKind)
	}
	if refs[0].RawTargetText != "/users" {
		t.Errorf("expected raw target '/users', got %q", refs[0].RawTargetText)
	}

	// Check second route
	if arts[1].Name != "POST /api/items" {
		t.Errorf("expected 'POST /api/items', got %q", arts[1].Name)
	}

	// Check third route
	if arts[2].Name != "DELETE /api/items/:id" {
		t.Errorf("expected 'DELETE /api/items/:id', got %q", arts[2].Name)
	}
}

func TestExtractRoutes_NestJS(t *testing.T) {
	content := `@Controller('users')
export class UsersController {
  @Get('/users/:id')
  findOne(@Param('id') id: string) {}

  @Post('/users')
  create(@Body() dto: CreateUserDto) {}
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "typescript")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}
	if arts[0].Name != "GET /users/:id" {
		t.Errorf("expected 'GET /users/:id', got %q", arts[0].Name)
	}
	if arts[1].Name != "POST /users" {
		t.Errorf("expected 'POST /users', got %q", arts[1].Name)
	}
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}
}

func TestExtractRoutes_NextJS(t *testing.T) {
	content := `export async function GET(req) {
  return Response.json({ users: [] })
}

export function POST(req) {
  return Response.json({ ok: true })
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "typescript")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}
	if arts[0].Name != "GET " {
		t.Errorf("expected 'GET ', got %q", arts[0].Name)
	}
	if arts[1].Name != "POST " {
		t.Errorf("expected 'POST ', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `// app.get('/commented-out', handler)
app.get('/real-route', handler)
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented route excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET /real-route" {
		t.Errorf("expected 'GET /real-route', got %q", arts[0].Name)
	}
}

func TestExtractRoutes_Multiple(t *testing.T) {
	content := `app.get('/users', listUsers)
app.post('/users', createUser)
app.put('/users/:id', updateUser)
app.delete('/users/:id', deleteUser)
app.patch('/users/:id', patchUser)
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 5 {
		t.Fatalf("expected 5 route refs, got %d", len(refs))
	}
	if len(arts) != 5 {
		t.Fatalf("expected 5 route artifacts, got %d", len(arts))
	}

	expected := []string{"GET /users", "POST /users", "PUT /users/:id", "DELETE /users/:id", "PATCH /users/:id"}
	for i, exp := range expected {
		if arts[i].Name != exp {
			t.Errorf("route %d: expected %q, got %q", i, exp, arts[i].Name)
		}
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
