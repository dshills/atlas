package javaextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractTestReferences_DirectMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "processOrder", QualifiedName: "com.app.OrderService.processOrder", SymbolKind: "method"},
		{Name: "processOrder", QualifiedName: "com.app.OrderServiceTest.processOrder", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "com.app.OrderService")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "tests" {
		t.Errorf("expected reference kind 'tests', got %q", refs[0].ReferenceKind)
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
	if refs[0].FromSymbolName != "com.app.OrderServiceTest.processOrder" {
		t.Errorf("expected from 'com.app.OrderServiceTest.processOrder', got %q", refs[0].FromSymbolName)
	}
	if refs[0].ToSymbolName != "com.app.OrderService.processOrder" {
		t.Errorf("expected to 'com.app.OrderService.processOrder', got %q", refs[0].ToSymbolName)
	}
	if refs[0].RawTargetText != "processOrder" {
		t.Errorf("expected raw target 'processOrder', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_TestPrefixMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "ProcessOrder", QualifiedName: "com.app.Service.ProcessOrder", SymbolKind: "method"},
		{Name: "TestProcessOrder", QualifiedName: "com.app.ServiceTest.TestProcessOrder", SymbolKind: "test", StartLine: 10},
	}

	refs := extractTestReferences(symbols, "com.app.Service")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].ToSymbolName != "com.app.Service.ProcessOrder" {
		t.Errorf("expected to 'com.app.Service.ProcessOrder', got %q", refs[0].ToSymbolName)
	}
}

func TestExtractTestReferences_LowercasePrefix(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "handleRequest", QualifiedName: "com.app.Handler.handleRequest", SymbolKind: "method"},
		{Name: "testHandleRequest", QualifiedName: "com.app.HandlerTest.testHandleRequest", SymbolKind: "test", StartLine: 15},
	}

	refs := extractTestReferences(symbols, "com.app.Handler")

	if len(refs) != 1 {
		t.Fatalf("expected 1 test reference, got %d", len(refs))
	}
	if refs[0].RawTargetText != "handleRequest" {
		t.Errorf("expected raw target 'handleRequest', got %q", refs[0].RawTargetText)
	}
}

func TestExtractTestReferences_NoMatch(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "processOrder", QualifiedName: "com.app.Service.processOrder", SymbolKind: "method"},
		{Name: "testSomethingElse", QualifiedName: "com.app.ServiceTest.testSomethingElse", SymbolKind: "test", StartLine: 5},
	}

	refs := extractTestReferences(symbols, "com.app.Service")

	if len(refs) != 0 {
		t.Errorf("expected 0 test references for unmatched test, got %d", len(refs))
	}
}

func TestExtractTestReferences_MultipleMatches(t *testing.T) {
	symbols := []extractor.SymbolRecord{
		{Name: "createUser", QualifiedName: "com.app.UserService.createUser", SymbolKind: "method"},
		{Name: "deleteUser", QualifiedName: "com.app.UserService.deleteUser", SymbolKind: "method"},
		{Name: "testCreateUser", QualifiedName: "com.app.UserServiceTest.testCreateUser", SymbolKind: "test", StartLine: 5},
		{Name: "testDeleteUser", QualifiedName: "com.app.UserServiceTest.testDeleteUser", SymbolKind: "test", StartLine: 15},
	}

	refs := extractTestReferences(symbols, "com.app.UserService")

	if len(refs) != 2 {
		t.Fatalf("expected 2 test references, got %d", len(refs))
	}
}
