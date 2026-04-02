package swiftextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractSQLArtifacts_NSPredicate(t *testing.T) {
	content := `func fetchUsers() {
    let predicate = NSPredicate(format: "age > 18 AND name != nil")
    let request = NSFetchRequest<User>(entityName: "User")
    request.predicate = predicate
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractSQLArtifacts(content, lines, "Models/User.swift", codeLines)

	hasSQLArtifact := false
	for _, a := range arts {
		if a.ArtifactKind == "sql_query" {
			hasSQLArtifact = true
			break
		}
	}
	if !hasSQLArtifact {
		t.Error("expected sql_query artifact for NSPredicate")
	}

	hasTouchesTable := false
	for _, r := range refs {
		if r.ReferenceKind == "touches_table" {
			hasTouchesTable = true
			break
		}
	}
	if !hasTouchesTable {
		t.Error("expected touches_table reference for NSPredicate")
	}
}

func TestExtractSQLArtifacts_GRDBFilter(t *testing.T) {
	content := `func getActiveUsers(db: Database) throws -> [User] {
    return try User.filter(sql: "SELECT * FROM users WHERE active = 1").fetchAll(db)
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractSQLArtifacts(content, lines, "DB/Queries.swift", codeLines)

	hasSQLArtifact := false
	for _, a := range arts {
		if a.ArtifactKind == "sql_query" {
			hasSQLArtifact = true
			break
		}
	}
	if !hasSQLArtifact {
		t.Error("expected sql_query artifact for GRDB filter")
	}

	hasTouchesTable := false
	for _, r := range refs {
		if r.ReferenceKind == "touches_table" {
			hasTouchesTable = true
			break
		}
	}
	if !hasTouchesTable {
		t.Error("expected touches_table reference for GRDB filter")
	}
}

func TestExtractSQLArtifacts_MigrationFile(t *testing.T) {
	content := `struct CreateUserMigration: Migration {
    func prepare(on database: Database) -> EventLoopFuture<Void> {
        database.schema("users").create()
    }
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractSQLArtifacts(content, lines, "Migrations/CreateUserMigration.swift", codeLines)

	hasMigration := false
	for _, a := range arts {
		if a.ArtifactKind == "migration" {
			hasMigration = true
			break
		}
	}
	if !hasMigration {
		t.Error("expected migration artifact for file with Migration in path")
	}

	hasMigrates := false
	for _, r := range refs {
		if r.ReferenceKind == "migrates" {
			hasMigrates = true
			break
		}
	}
	if !hasMigrates {
		t.Error("expected migrates reference for file with Migration in path")
	}
}

func TestExtractSQLArtifacts_CommentExcluded(t *testing.T) {
	content := `func main() {
    // let q = "SELECT * FROM secret_table WHERE x = 1"
    let x = 42
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "swift")

	refs, arts := extractSQLArtifacts(content, lines, "main.swift", codeLines)

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
