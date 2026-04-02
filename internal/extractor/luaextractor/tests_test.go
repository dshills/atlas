package luaextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTestReferences_TestPrefixMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "create_user", QualifiedName: "app.users.create_user", SymbolKind: "function"},
		{Name: "test_create_user", QualifiedName: "tests.test_users.test_create_user", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "app.users")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "tests" {
		t.Errorf("expected reference kind 'tests', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "likely" {
		t.Errorf("expected confidence 'likely', got %q", refs[0].Confidence)
	}
	if refs[0].FromSymbolName != "tests.test_users.test_create_user" {
		t.Errorf("expected from 'tests.test_users.test_create_user', got %q", refs[0].FromSymbolName)
	}
	if refs[0].ToSymbolName != "app.users.create_user" {
		t.Errorf("expected to 'app.users.create_user', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "create_user" {
		t.Errorf("expected raw target 'create_user', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_TestClassPrefix(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "User", QualifiedName: "app.models.User", SymbolKind: "function"},
		{Name: "TestUser", QualifiedName: "tests.test_models.TestUser", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "app.models")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ToSymbolName != "app.models.User" {
		t.Errorf("expected to 'app.models.User', got %q", refs[0].ToSymbolName)
	}
}

func TestExtractTestReferences_NoMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "create_user", QualifiedName: "app.users.create_user", SymbolKind: "function"},
		{Name: "test_something_unrelated", QualifiedName: "tests.test_misc.test_something_unrelated", SymbolKind: "test", StartLine: 1},
	}

	refs := extractTestReferences(symbols, "app.users")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references for unmatched test, got %d", len(refs))
	}
}

func TestExtractTestReferences_BustedSkipped(t *testing.T) {
	// Busted describe/it block test names contain underscores but don't start with test
	symbols := []extractor.SymbolRecord{
		{Name: "create_user", QualifiedName: "app.users.create_user", SymbolKind: "function"},
		{Name: "should_create_a_user", QualifiedName: "tests.spec.should_create_a_user", SymbolKind: "test", StartLine: 3},
	}

	refs := extractTestReferences(symbols, "app.users")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references for busted it-block, got %d", len(refs))
	}
}

func TestExtractTestReferences_MultipleMatches(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "create_user", QualifiedName: "app.users.create_user", SymbolKind: "function"},
		{Name: "delete_user", QualifiedName: "app.users.delete_user", SymbolKind: "function"},
		{Name: "test_create_user", QualifiedName: "tests.test_users.test_create_user", SymbolKind: "test", StartLine: 5},
		{Name: "test_delete_user", QualifiedName: "tests.test_users.test_delete_user", SymbolKind: "test", StartLine: 12},
	}

	refs := extractTestReferences(symbols, "app.users")

	if len(refs) != 2 {
		t.Fatalf("expected 2 test references, got %d", len(refs))
	}
}
