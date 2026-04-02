package swiftextractor

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractSwiftBasic(t *testing.T) {
	ext := New()
	content := `import Foundation
import UIKit

public class UserService {
    public var name: String = ""
    private let id: Int = 0

    public func createUser(name: String) {
        // implementation
    }

    private func validate(email: String) -> Bool {
        return email.contains("@")
    }
}

public struct Point {
    var x: Double
    var y: Double

    func distance() -> Double {
        return (x * x + y * y).squareRoot()
    }
}

enum Direction {
    case north
    case south
    case east
    case west
}

protocol Drawable {
    func draw()
}

extension String {
    func reversed() -> String {
        return String(self.reversed())
    }
}

func topLevelHelper() {
    // top-level function
}
`

	req := extractor.ExtractRequest{
		FilePath: "Sources/Models/UserService.swift",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Check package
	if result.Package.Name != "UserService" {
		t.Errorf("expected package name 'UserService', got %q", result.Package.Name)
	}
	if result.Package.ImportPath != "Sources.Models.UserService" {
		t.Errorf("expected import path 'Sources.Models.UserService', got %q", result.Package.ImportPath)
	}
	if result.Package.Language != "swift" {
		t.Errorf("expected language 'swift', got %q", result.Package.Language)
	}

	// Check imports
	importCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "imports" {
			importCount++
		}
	}
	if importCount != 2 {
		t.Errorf("expected 2 imports, got %d", importCount)
	}

	// Check symbols by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	if kinds["class"] != 1 {
		t.Errorf("expected 1 class, got %d", kinds["class"])
	}
	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["protocol"] != 1 {
		t.Errorf("expected 1 protocol, got %d", kinds["protocol"])
	}
	if kinds["extension"] != 1 {
		t.Errorf("expected 1 extension, got %d", kinds["extension"])
	}
	if kinds["function"] != 1 {
		t.Errorf("expected 1 top-level function, got %d", kinds["function"])
	}
	if kinds["method"] < 4 {
		t.Errorf("expected at least 4 methods, got %d", kinds["method"])
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}
	if kinds["property"] < 2 {
		t.Errorf("expected at least 2 properties, got %d", kinds["property"])
	}

	// Check stable ID format
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "swift:") {
			t.Errorf("expected stable ID to start with swift:, got %s", sym.StableID)
		}
	}

	// Check file parse status
	if result.File.ParseStatus != "ok" {
		t.Errorf("expected parse status 'ok', got %q", result.File.ParseStatus)
	}
}

func TestExtractSwiftTestDetection(t *testing.T) {
	ext := New()
	content := `import XCTest

class UserServiceTests: XCTestCase {
    func testCreateUser() {
        // XCTest style
    }

    func testValidateEmail() {
        // XCTest style
    }

    func helperMethod() {
        // not a test
    }
}

import Testing

struct CalculatorTests {
    @Test
    func addition() {
        // Swift Testing style
    }

    @Test
    public func subtraction() {
        // Swift Testing style
    }

    func helperSetup() {
        // not a test
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "Tests/UserServiceTests.swift",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	testCount := 0
	methodCount := 0
	for _, sym := range result.Symbols {
		switch sym.SymbolKind {
		case "test":
			testCount++
		case "method":
			methodCount++
		}
	}

	if testCount != 4 {
		t.Errorf("expected 4 test symbols, got %d", testCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}
	if methodCount != 2 {
		t.Errorf("expected 2 non-test methods, got %d", methodCount)
		for _, sym := range result.Symbols {
			if sym.SymbolKind == "method" {
				t.Logf("  method: %s line=%d", sym.Name, sym.StartLine)
			}
		}
	}
}

func TestExtractSwiftVisibility(t *testing.T) {
	ext := New()
	content := `public class PublicClass {
    public func publicMethod() {}
    open func openMethod() {}
    private func privateMethod() {}
    fileprivate func fileprivateMethod() {}
    internal func internalMethod() {}
    func defaultMethod() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "Sources/Example.swift",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Class should be exported (public)
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "class" && sym.Name == "PublicClass" {
			if sym.Visibility != "exported" {
				t.Errorf("expected PublicClass to be exported, got %s", sym.Visibility)
			}
		}
	}

	exported := 0
	unexported := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind != "method" {
			continue
		}
		switch sym.Visibility {
		case "exported":
			exported++
		case "unexported":
			unexported++
		}
	}

	// public and open are exported (2), rest are unexported (4)
	if exported != 2 {
		t.Errorf("expected 2 exported methods, got %d", exported)
		for _, sym := range result.Symbols {
			if sym.SymbolKind == "method" {
				t.Logf("  method: %s visibility=%s line=%d", sym.Name, sym.Visibility, sym.StartLine)
			}
		}
	}
	if unexported != 4 {
		t.Errorf("expected 4 unexported methods, got %d", unexported)
	}
}

func TestExtractSwiftQualifiedNames(t *testing.T) {
	ext := New()
	content := `public class OrderService {
    public func placeOrder() {}
    private func validate() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "Sources/Services/OrderService.swift",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	expectedNames := map[string]bool{
		"Sources.Services.OrderService.OrderService":            false,
		"Sources.Services.OrderService.OrderService.placeOrder": false,
		"Sources.Services.OrderService.OrderService.validate":   false,
	}

	for _, sym := range result.Symbols {
		if _, ok := expectedNames[sym.QualifiedName]; ok {
			expectedNames[sym.QualifiedName] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected qualified name %q not found", name)
			for _, sym := range result.Symbols {
				t.Logf("  found: %s (kind=%s)", sym.QualifiedName, sym.SymbolKind)
			}
		}
	}

	// All qualified names should contain the module prefix
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.QualifiedName, "Sources.Services.OrderService.") {
			t.Errorf("unexpected qualified name format: %s", sym.QualifiedName)
		}
	}
}

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `import Foundation
import UIKit
import Combine

public class UserService {
    public var users: [User] = []
    private let maxRetries: Int = 3

    public func findAll() -> [User] {
        return users
    }

    public func findById(id: String) -> User? {
        return nil
    }

    private func validateUser(user: User) {
        // validation
    }
}

public protocol UserRepository {
    func save(user: User)
    func findById(id: String) -> User?
}

public enum UserRole {
    case admin
    case user
    case guest
}

public struct UserDTO {
    let id: String
    let name: String
}

extension UserService {
    func debugDescription() -> String {
        return "UserService"
    }
}

let MAX_USERS = 1000

func globalHelper() -> Bool {
    return true
}
`

	req := extractor.ExtractRequest{
		FilePath: "Sources/Services/UserService.swift",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify file record
	if result.File == nil || result.File.ParseStatus != "ok" {
		t.Fatal("expected file record with parse status 'ok'")
	}

	// Verify package record
	if result.Package == nil {
		t.Fatal("expected package record")
	}
	if result.Package.ImportPath != "Sources.Services.UserService" {
		t.Errorf("expected import path 'Sources.Services.UserService', got %q", result.Package.ImportPath)
	}

	// Count imports
	importCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "imports" {
			importCount++
			if ref.Confidence != "exact" {
				t.Errorf("expected import confidence 'exact', got %q", ref.Confidence)
			}
			if ref.RawTargetText == "" {
				t.Errorf("expected non-empty RawTargetText for import at line %d", ref.Line)
			}
		}
	}
	if importCount != 3 {
		t.Errorf("expected 3 imports, got %d", importCount)
	}

	// Count symbols by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	if kinds["class"] != 1 {
		t.Errorf("expected 1 class, got %d", kinds["class"])
	}
	if kinds["protocol"] != 1 {
		t.Errorf("expected 1 protocol, got %d", kinds["protocol"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["extension"] != 1 {
		t.Errorf("expected 1 extension, got %d", kinds["extension"])
	}
	if kinds["function"] != 1 {
		t.Errorf("expected 1 top-level function, got %d", kinds["function"])
	}
	if kinds["method"] < 3 {
		t.Errorf("expected at least 3 methods, got %d", kinds["method"])
	}
	if kinds["property"] < 2 {
		t.Errorf("expected at least 2 properties, got %d", kinds["property"])
	}

	// Verify all symbols have stable IDs starting with swift:
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "swift:") {
			t.Errorf("expected stable ID to start with swift:, got %s", sym.StableID)
		}
	}

	// Verify all symbols have qualified names with module prefix
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.QualifiedName, "Sources.Services.UserService.") {
			t.Errorf("unexpected qualified name format: %s", sym.QualifiedName)
		}
	}

	// Verify methods/properties have parent symbol IDs
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" || sym.SymbolKind == "property" {
			if sym.ParentSymbolID == "" {
				t.Errorf("expected parent symbol ID for %s %q", sym.SymbolKind, sym.Name)
			}
		}
	}

	// Verify StartLine and EndLine are set
	for _, sym := range result.Symbols {
		if sym.StartLine == 0 {
			t.Errorf("symbol %q has StartLine 0", sym.Name)
		}
		if sym.EndLine == 0 {
			t.Errorf("symbol %q has EndLine 0", sym.Name)
		}
	}
}

func TestSwiftSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"File.swift":       true,
		"Main.swift":       true,
		"Test.SWIFT":       true,
		"file.go":          false,
		"file.py":          false,
		"file.java":        false,
		"file.ts":          false,
		"Makefile":         false,
		"Package.swift.bk": false,
	}
	for path, want := range cases {
		if got := ext.Supports(path); got != want {
			t.Errorf("Supports(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSwiftLanguage(t *testing.T) {
	ext := New()
	if ext.Language() != "swift" {
		t.Errorf("expected language swift, got %s", ext.Language())
	}
}
