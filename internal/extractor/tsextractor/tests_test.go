package tsextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTestReferences_DescribeMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "validateEmail", QualifiedName: "src/user.validateEmail", SymbolKind: "function"},
		{Name: "validateEmail", QualifiedName: "src/user.test.validateEmail", SymbolKind: "test", StartLine: 3},
	}

	refs := extractTestReferences(symbols, "src/user")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "tests" {
		t.Errorf("expected reference kind 'tests', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
	if refs[0].FromSymbolName != "src/user.test.validateEmail" {
		t.Errorf("expected from 'src/user.test.validateEmail', got %q", refs[0].FromSymbolName)
	}
	if refs[0].ToSymbolName != "src/user.validateEmail" {
		t.Errorf("expected to 'src/user.validateEmail', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "validateEmail" {
		t.Errorf("expected raw target 'validateEmail', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_TestPrefixMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "Foo", QualifiedName: "src/mod.Foo", SymbolKind: "function"},
		{Name: "TestFoo", QualifiedName: "src/mod.TestFoo", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "src/mod")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ToSymbolName != "src/mod.Foo" {
		t.Errorf("expected to 'src/mod.Foo', got %q", refs[0].ToSymbolName)
	}
}

func TestExtractTestReferences_LowercasePrefix(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "handleRequest", QualifiedName: "src/mod.handleRequest", SymbolKind: "function"},
		{Name: "testHandleRequest", QualifiedName: "src/mod.testHandleRequest", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "src/mod")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].RawTargetText != "handleRequest" {
		t.Errorf("expected raw target 'handleRequest', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_NoMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "validateEmail", QualifiedName: "src/user.validateEmail", SymbolKind: "function"},
		{Name: "should handle errors", QualifiedName: "src/user.test.should_handle_errors", SymbolKind: "test", StartLine: 3},
	}

	refs := extractTestReferences(symbols, "src/user")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references for unmatched test, got %d", len(refs))
	}
}

func TestExtractTestReferences_MultipleMatches(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "createUser", QualifiedName: "src/user.createUser", SymbolKind: "function"},
		{Name: "deleteUser", QualifiedName: "src/user.deleteUser", SymbolKind: "function"},
		{Name: "createUser", QualifiedName: "src/user.test.createUser", SymbolKind: "test", StartLine: 3},
		{Name: "deleteUser", QualifiedName: "src/user.test.deleteUser", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "src/user")

	if len(refs) != 2 {
		t.Fatalf("expected 2 test references, got %d", len(refs))
	}
}
