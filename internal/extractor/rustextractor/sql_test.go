package rustextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractSQLArtifacts_StringSQL(t *testing.T) {
	content := `fn get_user(pool: &PgPool, id: i32) {
    let row = sqlx::query("SELECT * FROM users WHERE id = $1")
        .bind(id)
        .fetch_one(pool)
        .await;
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractSQLArtifacts(content, lines, "src/db.rs", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 sql artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "sql_query" {
		t.Errorf("expected artifact kind 'sql_query', got %q", arts[0].ArtifactKind)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "touches_table" {
		t.Errorf("expected reference kind 'touches_table', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
}

func TestExtractSQLArtifacts_SqlxQueryMacro(t *testing.T) {
	content := `fn create_item(pool: &PgPool) {
    sqlx::query!("INSERT INTO items (name) VALUES ($1)")
        .bind("widget")
        .execute(pool)
        .await;
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractSQLArtifacts(content, lines, "src/items.rs", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 sql artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "sql_query" {
		t.Errorf("expected artifact kind 'sql_query', got %q", arts[0].ArtifactKind)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "touches_table" {
		t.Errorf("expected reference kind 'touches_table', got %q", refs[0].ReferenceKind)
	}
}

func TestExtractSQLArtifacts_MigrationFile(t *testing.T) {
	content := `CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractSQLArtifacts(content, lines, "migrations/001_create_users.sql", codeLines)

	// Should have at least the migration artifact
	hasMigration := false
	for _, a := range arts {
		if a.ArtifactKind == "migration" {
			hasMigration = true
			break
		}
	}
	if !hasMigration {
		t.Error("expected migration artifact for file in migrations/ directory")
	}

	hasMigrates := false
	for _, r := range refs {
		if r.ReferenceKind == "migrates" {
			hasMigrates = true
			break
		}
	}
	if !hasMigrates {
		t.Error("expected migrates reference for file in migrations/ directory")
	}
}

func TestExtractSQLArtifacts_CommentExcluded(t *testing.T) {
	content := `fn main() {
    // sqlx::query!("SELECT * FROM secret_table WHERE x = 1")
    let x = 42;
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractSQLArtifacts(content, lines, "src/main.rs", codeLines)

	for _, a := range arts {
		if a.ArtifactKind == "sql_query" {
			t.Error("expected commented SQL to be excluded, but got sql_query artifact")
		}
	}
	for _, r := range refs {
		if r.ReferenceKind == "touches_table" {
			t.Error("expected commented SQL to be excluded, but got touches_table reference")
		}
	}
}
