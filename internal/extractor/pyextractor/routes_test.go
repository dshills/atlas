package pyextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractRoutes_Flask(t *testing.T) {
	content := `from flask import Flask
app = Flask(__name__)

@app.route('/users')
def list_users():
    pass

@app.get('/items')
def get_items():
    pass
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 route artifacts, got %d", len(arts))
	}

	if arts[0].Name != "ANY /users" {
		t.Errorf("expected 'ANY /users', got %q", arts[0].Name)
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

	if arts[1].Name != "GET /items" {
		t.Errorf("expected 'GET /items', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_FastAPI(t *testing.T) {
	content := `from fastapi import APIRouter
router = APIRouter()

@router.post('/api/items')
async def create_item(item: Item):
    pass
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref, got %d", len(refs))
	}
	if arts[0].Name != "POST /api/items" {
		t.Errorf("expected 'POST /api/items', got %q", arts[0].Name)
	}
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}
}

func TestExtractRoutes_Django(t *testing.T) {
	content := `from django.urls import path

urlpatterns = [
    path('users/', views.user_list),
    path('users/<int:pk>/', views.user_detail),
]
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 route refs, got %d", len(refs))
	}
	if arts[0].Name != "ANY users/" {
		t.Errorf("expected 'ANY users/', got %q", arts[0].Name)
	}
	if arts[1].Name != "ANY users/<int:pk>/" {
		t.Errorf("expected 'ANY users/<int:pk>/', got %q", arts[1].Name)
	}
}

func TestExtractRoutes_CommentedOut(t *testing.T) {
	content := `# @app.route('/hidden')
@app.get('/visible')
def visible():
    pass
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractRoutes(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 route ref (commented route excluded), got %d", len(refs))
	}
	if arts[0].Name != "GET /visible" {
		t.Errorf("expected 'GET /visible', got %q", arts[0].Name)
	}
}
