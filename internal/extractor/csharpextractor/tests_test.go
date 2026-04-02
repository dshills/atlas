package csharpextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTestReferences_StripTestPrefix(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "CreateUser", QualifiedName: "App.UserService.CreateUser", SymbolKind: "method", StartLine: 5},
		{Name: "TestCreateUser", QualifiedName: "App.UserServiceTest.TestCreateUser", SymbolKind: "test", StartLine: 15},
	}

	refs := extractTestReferences(symbols, "App")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test ref, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "tests" {
		t.Errorf("expected reference kind 'tests', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "likely" {
		t.Errorf("expected confidence 'likely', got %q", refs[0].Confidence)
	}
	if refs[0].RawTargetText != "CreateUser" {
		t.Errorf("expected raw target 'CreateUser', got %q", refs[0].RawTargetText)
	}
	if refs[0].ToSymbolName != "App.CreateUser" {
		t.Errorf("expected to symbol 'App.CreateUser', got %q", refs[0].ToSymbolName)
	}
}

func TestExtractTestReferences_LowercaseFirst(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "processOrder", QualifiedName: "App.OrderService.processOrder", SymbolKind: "method", StartLine: 5},
		{Name: "TestProcessOrder", QualifiedName: "App.OrderServiceTest.TestProcessOrder", SymbolKind: "test", StartLine: 15},
	}

	refs := extractTestReferences(symbols, "App")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test ref, got %d", len(refs))
	}
	if refs[0].RawTargetText != "processOrder" {
		t.Errorf("expected raw target 'processOrder', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_DirectMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "Validate", QualifiedName: "App.Service.Validate", SymbolKind: "method", StartLine: 3},
		{Name: "Validate", QualifiedName: "App.ServiceTest.Validate", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "App")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test ref, got %d", len(refs))
	}
	if refs[0].RawTargetText != "Validate" {
		t.Errorf("expected raw target 'Validate', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_NoMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "CreateUser", QualifiedName: "App.UserService.CreateUser", SymbolKind: "method", StartLine: 5},
		{Name: "TestDeleteUser", QualifiedName: "App.UserServiceTest.TestDeleteUser", SymbolKind: "test", StartLine: 15},
	}

	refs := extractTestReferences(symbols, "App")

	if len(refs) != 0 {
		t.Errorf("expected 0 test refs (no match), got %d", len(refs))
	}
}

func TestExtractTestReferences_SkipNonTest(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "CreateUser", QualifiedName: "App.UserService.CreateUser", SymbolKind: "method", StartLine: 5},
		{Name: "DeleteUser", QualifiedName: "App.UserService.DeleteUser", SymbolKind: "method", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "App")

	if len(refs) != 0 {
		t.Errorf("expected 0 test refs (no test symbols), got %d", len(refs))
	}
}
