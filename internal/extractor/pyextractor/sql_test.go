package pyextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractSQLArtifacts_TripleQuotedSQL(t *testing.T) {
	content := `query = """SELECT * FROM users WHERE active = 1"""
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractSQLArtifacts(content, lines, "app/models.py", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 sql artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "sql_query" {
		t.Errorf("expected artifact kind 'sql_query', got %q", arts[0].ArtifactKind)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 sql ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "touches_table" {
		t.Errorf("expected reference kind 'touches_table', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
}

func TestExtractSQLArtifacts_StringSQL(t *testing.T) {
	content := `cursor.execute("INSERT INTO orders VALUES (?, ?, ?)")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractSQLArtifacts(content, lines, "app/db.py", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 sql artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "sql_query" {
		t.Errorf("expected artifact kind 'sql_query', got %q", arts[0].ArtifactKind)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 sql ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "touches_table" {
		t.Errorf("expected reference kind 'touches_table', got %q", refs[0].ReferenceKind)
	}
}

func TestExtractSQLArtifacts_MigrationFile(t *testing.T) {
	content := `# migration script
def upgrade():
    pass
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractSQLArtifacts(content, lines, "db/migrations/001_init.py", codeLines)

	if len(arts) != 1 {
		t.Fatalf("expected 1 migration artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "migration" {
		t.Errorf("expected artifact kind 'migration', got %q", arts[0].ArtifactKind)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 migration ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "migrates" {
		t.Errorf("expected reference kind 'migrates', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "exact" {
		t.Errorf("expected confidence 'exact', got %q", refs[0].Confidence)
	}
}

func TestExtractSQLArtifacts_CommentExcluded(t *testing.T) {
	content := `# query = "SELECT * FROM users WHERE active = 1"
real = "not a sql query at all"
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "python")

	refs, arts := extractSQLArtifacts(content, lines, "app/models.py", codeLines)

	if len(arts) != 0 {
		t.Fatalf("expected 0 sql artifacts for commented code, got %d", len(arts))
	}
	if len(refs) != 0 {
		t.Fatalf("expected 0 sql refs for commented code, got %d", len(refs))
	}
}
