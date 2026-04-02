package tsextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractSQLArtifacts_TemplateLiteral(t *testing.T) {
	content := "const q = `SELECT * FROM users WHERE id = ?`\n"
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractSQLArtifacts(content, lines, "src/db.js", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 sql_query artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "sql_query" {
		t.Errorf("expected artifact kind 'sql_query', got %q", arts[0].ArtifactKind)
	}
	if arts[0].Name != "SELECT" {
		t.Errorf("expected name 'SELECT', got %q", arts[0].Name)
	}
	if arts[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", arts[0].Confidence)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "touches_table" {
		t.Errorf("expected reference kind 'touches_table', got %q", refs[0].ReferenceKind)
	}
}

func TestExtractSQLArtifacts_StringLiteral(t *testing.T) {
	content := `const q = "INSERT INTO orders (id, name) VALUES (?, ?)"
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractSQLArtifacts(content, lines, "src/orders.js", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 sql_query artifact, got %d", len(arts))
	}
	if arts[0].Name != "INSERT" {
		t.Errorf("expected name 'INSERT', got %q", arts[0].Name)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
}

func TestExtractSQLArtifacts_MigrationFile(t *testing.T) {
	content := "module.exports = { up: () => {} }\n"
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractSQLArtifacts(content, lines, "db/migrations/001_create_users.js", codeLines)

	// Should have migration artifact
	found := false
	for _, a := range arts {
		if a.ArtifactKind == "migration" {
			found = true
			if a.Name != "db/migrations/001_create_users.js" {
				t.Errorf("expected migration name to be file path, got %q", a.Name)
			}
			if a.Confidence != "exact" {
				t.Errorf("expected confidence 'exact', got %q", a.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected migration artifact")
	}

	foundRef := false
	for _, r := range refs {
		if r.ReferenceKind == "migrates" {
			foundRef = true
		}
	}
	if !foundRef {
		t.Error("expected migrates reference")
	}
}

func TestExtractSQLArtifacts_CommentExcluded(t *testing.T) {
	content := "// const q = `SELECT * FROM users WHERE id = ?`\n"
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractSQLArtifacts(content, lines, "src/db.js", codeLines)

	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts for commented SQL, got %d", len(arts))
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for commented SQL, got %d", len(refs))
	}
}

func TestExtractSQLArtifacts_ShortString(t *testing.T) {
	content := `const q = "SELECT 1"
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	refs, arts := extractSQLArtifacts(content, lines, "src/db.js", codeLines)

	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts for short SQL string, got %d", len(arts))
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for short SQL string, got %d", len(refs))
	}
}

func TestExtractSQLArtifacts_CreateTable(t *testing.T) {
	content := "const q = `CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(255))`\n"
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "javascript")

	_, arts := extractSQLArtifacts(content, lines, "src/db.js", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].Name != "CREATE TABLE" {
		t.Errorf("expected name 'CREATE TABLE', got %q", arts[0].Name)
	}
}
