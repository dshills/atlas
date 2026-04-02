package rustextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractRoutes_ActixRocketAttribute(t *testing.T) {
	content := `use actix_web::{get, post};

#[get("/users/{id}")]
async fn get_user(path: web::Path<u32>) -> impl Responder {
    HttpResponse::Ok().body("user")
}

#[post("/users")]
async fn create_user(body: web::Json<User>) -> impl Responder {
    HttpResponse::Created().body("created")
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}

	if arts[0].Name != "GET /users/{id}" {
		t.Errorf("expected 'GET /users/{id}', got %q", arts[0].Name)
	}
	if arts[0].ArtifactKind != "route" {
		t.Errorf("expected artifact kind 'route', got %q", arts[0].ArtifactKind)
	}
	if refs[0].ReferenceKind != "registers_route" {
		t.Errorf("expected reference kind 'registers_route', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}

	if arts[1].Name != "POST /users" {
		t.Errorf("expected 'POST /users', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_Axum(t *testing.T) {
	content := `use axum::{routing::get, Router};

let app = Router::new()
    .route("/users", get(list_users))
    .route("/users/:id", get(get_user));
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

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
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}

	if arts[1].Name != "GET /users/:id" {
		t.Errorf("expected 'GET /users/:id', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_ActixResource(t *testing.T) {
	content := `use actix_web::{web, App};

App::new()
    .route("/items", web::post().to(create_item))
    .route("/items/{id}", web::get().to(get_item));
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}

	if arts[0].Name != "POST /items" {
		t.Errorf("expected 'POST /items', got %q", arts[0].Name)
	}
	if !strings.Contains(arts[0].DataJSON, `"handler":"create_item"`) {
		t.Errorf("expected handler 'create_item' in DataJSON, got %q", arts[0].DataJSON)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}

	if arts[1].Name != "GET /items/{id}" {
		t.Errorf("expected 'GET /items/{id}', got %q", arts[1].Name)
	}
	if !strings.Contains(arts[1].DataJSON, `"handler":"get_item"`) {
		t.Errorf("expected handler 'get_item' in DataJSON, got %q", arts[1].DataJSON)
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `// #[get("/hidden")]
#[get("/visible")]
async fn visible() -> impl Responder {
    HttpResponse::Ok()
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented route excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET /visible" {
		t.Errorf("expected 'GET /visible', got %q", arts[0].Name)
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
