package luaextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractSQLArtifacts_BasicQuery(t *testing.T) {
	content := `local result = db:execute("SELECT * FROM users WHERE id = ?")
db:execute("INSERT INTO logs (msg) VALUES (?)")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractSQLArtifacts(content, lines, "app.lua", codeLines)

	if len(refs) != 2 {
		t.Fatalf("expected 2 SQL refs, got %d", len(refs))
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 SQL artifacts, got %d", len(arts))
	}

	if refs[0].ReferenceKind != "touches_table" {
		t.Errorf("expected reference kind 'touches_table', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
	if arts[0].ArtifactKind != "sql_query" {
		t.Errorf("expected artifact kind 'sql_query', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractSQLArtifacts_MigrationFile(t *testing.T) {
	content := `CREATE TABLE users (id INTEGER PRIMARY KEY)
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractSQLArtifacts(content, lines, "db/migration/001_create_users.lua", codeLines)

	// Should have migration artifact + SQL artifact
	hasMigration := false
	hasSQL := false
	for _, a := range arts {
		if a.ArtifactKind == "migration" {
			hasMigration = true
		}
		if a.ArtifactKind == "sql_query" {
			hasSQL = true
		}
	}
	if !hasMigration {
		t.Error("expected migration artifact")
	}
	if !hasSQL {
		t.Error("expected sql_query artifact")
	}

	hasMigrates := false
	for _, r := range refs {
		if r.ReferenceKind == "migrates" {
			hasMigrates = true
		}
	}
	if !hasMigrates {
		t.Error("expected migrates reference")
	}
}

func TestExtractSQLArtifacts_CommentedOut(t *testing.T) {
	content := `-- SELECT * FROM hidden_table
local result = db:execute("SELECT * FROM visible")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, _ := extractSQLArtifacts(content, lines, "app.lua", codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 SQL ref (commented excluded), got %d", len(refs))
	}
}

func TestExtractSQLArtifacts_NoDuplicates(t *testing.T) {
	content := `db:execute("SELECT id, name FROM users WHERE active = true")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, _ := extractSQLArtifacts(content, lines, "app.lua", codeLines)

	// Multiple SQL keywords on same line should produce only one reference
	if len(refs) != 1 {
		t.Fatalf("expected 1 SQL ref (deduped by line), got %d", len(refs))
	}
}
