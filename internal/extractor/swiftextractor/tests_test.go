package swiftextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTestReferences_DirectMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "validateEmail", QualifiedName: "Sources.User.validateEmail", SymbolKind: "function"},
		{Name: "validateEmail", QualifiedName: "Tests.UserTests.validateEmail", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "Sources.User")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "tests" {
		t.Errorf("expected reference kind 'tests', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "likely" {
		t.Errorf("expected confidence 'likely', got %q", refs[0].Confidence)
	}
	if refs[0].FromSymbolName != "Tests.UserTests.validateEmail" {
		t.Errorf("expected from 'Tests.UserTests.validateEmail', got %q", refs[0].FromSymbolName)
	}
	if refs[0].ToSymbolName != "Sources.User.validateEmail" {
		t.Errorf("expected to 'Sources.User.validateEmail', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "validateEmail" {
		t.Errorf("expected raw target 'validateEmail', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_TestPrefixMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "add", QualifiedName: "Sources.Math.add", SymbolKind: "function"},
		{Name: "testAdd", QualifiedName: "Tests.MathTests.testAdd", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "Sources.Math")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ToSymbolName != "Sources.Math.add" {
		t.Errorf("expected to 'Sources.Math.add', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "add" {
		t.Errorf("expected raw target 'add', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_TestCapitalPrefixMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "createUser", QualifiedName: "Sources.User.createUser", SymbolKind: "function"},
		{Name: "testCreateUser", QualifiedName: "Tests.UserTests.testCreateUser", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "Sources.User")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ToSymbolName != "Sources.User.createUser" {
		t.Errorf("expected to 'Sources.User.createUser', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "createUser" {
		t.Errorf("expected raw target 'createUser', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_NoMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "validateEmail", QualifiedName: "Sources.User.validateEmail", SymbolKind: "function"},
		{Name: "testSomethingUnrelated", QualifiedName: "Tests.UserTests.testSomethingUnrelated", SymbolKind: "test", StartLine: 3},
	}

	refs := extractTestReferences(symbols, "Sources.User")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references for unmatched test, got %d", len(refs))
	}
}

func TestExtractTestReferences_MultipleMatches(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "createUser", QualifiedName: "Sources.User.createUser", SymbolKind: "function"},
		{Name: "deleteUser", QualifiedName: "Sources.User.deleteUser", SymbolKind: "function"},
		{Name: "testCreateUser", QualifiedName: "Tests.UserTests.testCreateUser", SymbolKind: "test", StartLine: 3},
		{Name: "testDeleteUser", QualifiedName: "Tests.UserTests.testDeleteUser", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "Sources.User")

	if len(refs) != 2 {
		t.Fatalf("expected 2 test references, got %d", len(refs))
	}
}

func TestExtractTestReferences_SkipsNonTestSymbols(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "process", QualifiedName: "Sources.Mod.process", SymbolKind: "function"},
		{Name: "helper", QualifiedName: "Sources.Mod.helper", SymbolKind: "function"},
	}

	refs := extractTestReferences(symbols, "Sources.Mod")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references when no test symbols, got %d", len(refs))
	}
}
