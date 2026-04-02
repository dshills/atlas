package csharpextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractSQLArtifacts_Keywords(t *testing.T) {
	content := `var query = "SELECT * FROM Users WHERE Id = @id";
ExecuteNonQuery("INSERT INTO Orders (Id) VALUES (@id)");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractSQLArtifacts(content, lines, "src/UserRepo.cs", codeLines)

	if len(arts) != 2 {
		t.Fatalf("expected 2 sql_query artifacts, got %d", len(arts))
	}
	for _, a := range arts {
		if a.ArtifactKind != "sql_query" {
			t.Errorf("expected artifact kind 'sql_query', got %q", a.ArtifactKind)
		}
		if a.Confidence != "heuristic" {
			t.Errorf("expected confidence 'heuristic', got %q", a.Confidence)
		}
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	for _, r := range refs {
		if r.ReferenceKind != "touches_table" {
			t.Errorf("expected reference kind 'touches_table', got %q", r.ReferenceKind)
		}
	}
}

func TestExtractSQLArtifacts_EFCore(t *testing.T) {
	content := `var users = context.Users.FromSqlRaw("SELECT * FROM Users WHERE Active = 1");
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractSQLArtifacts(content, lines, "src/UserRepo.cs", codeLines)

	// Line has both SQL keyword match and FromSqlRaw; dedup should keep one per line
	if len(arts) < 1 {
		t.Fatalf("expected at least 1 sql_query artifact, got %d", len(arts))
	}
	if len(refs) < 1 {
		t.Fatalf("expected at least 1 ref, got %d", len(refs))
	}
}

func TestExtractSQLArtifacts_MigrationFile(t *testing.T) {
	content := `public class CreateUsersTable : Migration
{
    protected override void Up() { }
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractSQLArtifacts(content, lines, "Migrations/20210101_CreateUsersTable.cs", codeLines)

	foundMigration := false
	for _, a := range arts {
		if a.ArtifactKind == "migration" {
			foundMigration = true
			if a.Confidence != "heuristic" {
				t.Errorf("expected confidence 'heuristic', got %q", a.Confidence)
			}
		}
	}
	if !foundMigration {
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
	content := `// var query = "SELECT * FROM Users";
var active = true;
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractSQLArtifacts(content, lines, "src/UserRepo.cs", codeLines)

	if len(arts) != 0 {
		t.Errorf("expected 0 artifacts for commented SQL, got %d", len(arts))
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for commented SQL, got %d", len(refs))
	}
}

func TestExtractSQLArtifacts_DeduplicateByLine(t *testing.T) {
	content := `var q = "SELECT Id FROM Users WHERE Name = 'test'";
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs, arts := extractSQLArtifacts(content, lines, "src/Repo.cs", codeLines)

	// Multiple SQL keywords on the same line should produce only one artifact
	if len(arts) != 1 {
		t.Errorf("expected 1 artifact (deduplicated), got %d", len(arts))
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 ref (deduplicated), got %d", len(refs))
	}
}
