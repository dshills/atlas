package rustextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTestReferences_DirectMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "validate_email", QualifiedName: "src::user::validate_email", SymbolKind: "function"},
		{Name: "validate_email", QualifiedName: "src::user::tests::validate_email", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "src::user")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "tests" {
		t.Errorf("expected reference kind 'tests', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
	if refs[0].FromSymbolName != "src::user::tests::validate_email" {
		t.Errorf("expected from 'src::user::tests::validate_email', got %q", refs[0].FromSymbolName)
	}
	if refs[0].ToSymbolName != "src::user::validate_email" {
		t.Errorf("expected to 'src::user::validate_email', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "validate_email" {
		t.Errorf("expected raw target 'validate_email', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_TestPrefixMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "add", QualifiedName: "src::math::add", SymbolKind: "function"},
		{Name: "test_add", QualifiedName: "src::math::tests::test_add", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "src::math")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ToSymbolName != "src::math::add" {
		t.Errorf("expected to 'src::math::add', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "add" {
		t.Errorf("expected raw target 'add', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_NoMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "validate_email", QualifiedName: "src::user::validate_email", SymbolKind: "function"},
		{Name: "test_something_unrelated", QualifiedName: "src::user::tests::test_something_unrelated", SymbolKind: "test", StartLine: 3},
	}

	refs := extractTestReferences(symbols, "src::user")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references for unmatched test, got %d", len(refs))
	}
}

func TestExtractTestReferences_MultipleMatches(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "create_user", QualifiedName: "src::user::create_user", SymbolKind: "function"},
		{Name: "delete_user", QualifiedName: "src::user::delete_user", SymbolKind: "function"},
		{Name: "test_create_user", QualifiedName: "src::user::tests::test_create_user", SymbolKind: "test", StartLine: 3},
		{Name: "test_delete_user", QualifiedName: "src::user::tests::test_delete_user", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "src::user")

	if len(refs) != 2 {
		t.Fatalf("expected 2 test references, got %d", len(refs))
	}
}

func TestExtractTestReferences_SkipsNonTestSymbols(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "process", QualifiedName: "src::mod::process", SymbolKind: "function"},
		{Name: "helper", QualifiedName: "src::mod::helper", SymbolKind: "function"},
	}

	refs := extractTestReferences(symbols, "src::mod")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references when no test symbols, got %d", len(refs))
	}
}
