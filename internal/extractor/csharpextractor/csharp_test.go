package csharpextractor

import (
	"context"
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestExtractCSharpBasic(t *testing.T) {
	ext := New()
	content := `namespace MyApp.Services;

using System;
using System.Collections.Generic;
using static System.Math;

public class UserService {
    public const int MaxRetries = 3;
    public const string DefaultName = "unknown";

    public string Name { get; set; }
    public int Age { get; private set; }

    public void CreateUser(string name) {
        // implementation
    }

    private bool ValidateEmail(string email) {
        return email.Contains("@");
    }
}

public interface IRepository {
    void Save(object entity);
    object FindById(string id);
}

public enum Status {
    Active,
    Inactive,
    Suspended
}

public struct Point {
    public int X { get; set; }
    public int Y { get; set; }
}

public record Person(string FirstName, string LastName);
`

	req := extractor.ExtractRequest{
		FilePath: "src/MyApp/Services/UserService.cs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Check package
	if result.Package.Name != "Services" {
		t.Errorf("expected package name 'Services', got %q", result.Package.Name)
	}
	if result.Package.ImportPath != "MyApp.Services" {
		t.Errorf("expected import path 'MyApp.Services', got %q", result.Package.ImportPath)
	}
	if result.Package.Language != "csharp" {
		t.Errorf("expected language 'csharp', got %q", result.Package.Language)
	}

	// Check imports (3: System, System.Collections.Generic, System.Math)
	importCount := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "imports" {
			importCount++
		}
	}
	if importCount != 3 {
		t.Errorf("expected 3 imports, got %d", importCount)
	}

	// Check symbols by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	if kinds["class"] != 1 {
		t.Errorf("expected 1 class, got %d", kinds["class"])
	}
	if kinds["interface"] != 1 {
		t.Errorf("expected 1 interface, got %d", kinds["interface"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["record"] != 1 {
		t.Errorf("expected 1 record, got %d", kinds["record"])
	}
	if kinds["method"] < 2 {
		t.Errorf("expected at least 2 methods, got %d", kinds["method"])
	}
	if kinds["property"] < 2 {
		t.Errorf("expected at least 2 properties, got %d", kinds["property"])
	}
	if kinds["const"] < 2 {
		t.Errorf("expected at least 2 constants, got %d", kinds["const"])
	}

	// Check stable ID format
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "csharp:") {
			t.Errorf("expected stable ID to start with csharp:, got %s", sym.StableID)
		}
	}

	// Check file parse status
	if result.File.ParseStatus != "ok" {
		t.Errorf("expected parse status 'ok', got %q", result.File.ParseStatus)
	}
}

func TestExtractCSharpTestDetection(t *testing.T) {
	ext := New()
	content := `namespace MyApp.Tests;

using NUnit.Framework;
using Xunit;
using Microsoft.VisualStudio.TestTools.UnitTesting;

public class UserServiceTests {
    [Test]
    public void ShouldCreateUser() {
        // NUnit test
    }

    [Fact]
    public void ShouldValidateEmail() {
        // xUnit test
    }

    [Theory]
    public void ShouldHandleMultipleInputs() {
        // xUnit theory test
    }

    [TestMethod]
    public void ShouldDeleteUser() {
        // MSTest test
    }

    public void HelperMethod() {
        // not a test
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "tests/MyApp.Tests/UserServiceTests.cs",
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
	if methodCount != 1 {
		t.Errorf("expected 1 non-test method, got %d", methodCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}
}

func TestExtractCSharpVisibility(t *testing.T) {
	ext := New()
	content := `namespace MyApp;

public class Example {
    public void PublicMethod() {}
    internal void InternalMethod() {}
    private void PrivateMethod() {}
    protected void ProtectedMethod() {}
    void DefaultMethod() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/MyApp/Example.cs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Check class visibility
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "class" && sym.Name == "Example" {
			if sym.Visibility != "exported" {
				t.Errorf("expected public class Example to be exported, got %q", sym.Visibility)
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

	// public and internal are exported
	if exported != 2 {
		t.Errorf("expected 2 exported methods (public + internal), got %d", exported)
		for _, sym := range result.Symbols {
			if sym.SymbolKind == "method" {
				t.Logf("  method: %s visibility=%s", sym.Name, sym.Visibility)
			}
		}
	}
	// private, protected, and default are unexported
	if unexported != 3 {
		t.Errorf("expected 3 unexported methods (private + protected + default), got %d", unexported)
		for _, sym := range result.Symbols {
			if sym.SymbolKind == "method" {
				t.Logf("  method: %s visibility=%s", sym.Name, sym.Visibility)
			}
		}
	}
}

func TestExtractCSharpQualifiedNames(t *testing.T) {
	ext := New()
	content := `namespace MyApp.Services;

public class OrderService {
    public void PlaceOrder() {}
    private void Validate() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/MyApp/Services/OrderService.cs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	expectedNames := map[string]bool{
		"MyApp.Services.OrderService":            false,
		"MyApp.Services.OrderService.PlaceOrder": false,
		"MyApp.Services.OrderService.Validate":   false,
	}

	for _, sym := range result.Symbols {
		if _, ok := expectedNames[sym.QualifiedName]; ok {
			expectedNames[sym.QualifiedName] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected qualified name %q not found", name)
		}
	}

	// All qualified names should contain the namespace prefix
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.QualifiedName, "MyApp.Services.") {
			t.Errorf("unexpected qualified name format: %s", sym.QualifiedName)
		}
	}
}

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `namespace MyApp.Services;

using System;
using System.Collections.Generic;
using System.Threading.Tasks;

public class UserService {
    public const int MaxUsers = 1000;
    public const string ServiceName = "user-service";

    public string Name { get; set; }

    public List<User> FindAll() {
        return null;
    }

    public async Task<User> FindById(string id) {
        return null;
    }

    private void ValidateUser(User user) {
        // validation logic
    }
}

public interface IUserRepository {
    void Save(User user);
    User FindById(string id);
}

public enum UserRole {
    Admin,
    User,
    Guest
}

public struct Coordinate {
    public double Lat { get; set; }
    public double Lng { get; set; }
}

public record UserDto(string Name, string Email);
`

	req := extractor.ExtractRequest{
		FilePath: "src/MyApp/Services/UserService.cs",
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
	if result.Package.ImportPath != "MyApp.Services" {
		t.Errorf("expected import path 'MyApp.Services', got %q", result.Package.ImportPath)
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
	if kinds["interface"] != 1 {
		t.Errorf("expected 1 interface, got %d", kinds["interface"])
	}
	if kinds["enum"] != 1 {
		t.Errorf("expected 1 enum, got %d", kinds["enum"])
	}
	if kinds["struct"] != 1 {
		t.Errorf("expected 1 struct, got %d", kinds["struct"])
	}
	if kinds["record"] != 1 {
		t.Errorf("expected 1 record, got %d", kinds["record"])
	}
	if kinds["method"] < 3 {
		t.Errorf("expected at least 3 methods, got %d", kinds["method"])
	}
	if kinds["const"] < 2 {
		t.Errorf("expected at least 2 constants, got %d", kinds["const"])
	}

	// Verify all symbols have stable IDs starting with csharp:
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.StableID, "csharp:") {
			t.Errorf("expected stable ID to start with csharp:, got %s", sym.StableID)
		}
	}

	// Verify all symbols have qualified names with namespace prefix
	for _, sym := range result.Symbols {
		if !strings.HasPrefix(sym.QualifiedName, "MyApp.Services.") {
			t.Errorf("expected qualified name to start with MyApp.Services., got %s", sym.QualifiedName)
		}
	}

	// Verify methods and constants have parent symbol IDs
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" || sym.SymbolKind == "const" || sym.SymbolKind == "property" {
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

func TestCSharpSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"File.cs":        true,
		"UserService.cs": true,
		"Test.CS":        true,
		"file.go":        false,
		"file.py":        false,
		"file.java":      false,
		"file.ts":        false,
		"Makefile":       false,
		"App.csproj":     false,
		"MyClass.cs.bk":  false,
	}
	for path, want := range cases {
		if got := ext.Supports(path); got != want {
			t.Errorf("Supports(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestCSharpLanguage(t *testing.T) {
	ext := New()
	if ext.Language() != "csharp" {
		t.Errorf("expected language csharp, got %s", ext.Language())
	}
}

func TestCSharpSupportedKinds(t *testing.T) {
	ext := New()
	kinds := ext.SupportedKinds()
	expected := map[string]bool{
		"namespace": true, "class": true, "interface": true, "enum": true,
		"struct": true, "method": true, "property": true, "field": true,
		"const": true, "record": true, "test": true,
	}
	for _, k := range kinds {
		if !expected[k] {
			t.Errorf("unexpected kind: %s", k)
		}
		delete(expected, k)
	}
	for k := range expected {
		t.Errorf("missing expected kind: %s", k)
	}
}

func TestCSharpNamespaceFallback(t *testing.T) {
	ext := New()
	// No namespace declaration — should fallback to file path
	content := `public class Simple {
    public void DoSomething() {}
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/Simple.cs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if result.Package.ImportPath != "src.Simple" {
		t.Errorf("expected fallback import path 'src.Simple', got %q", result.Package.ImportPath)
	}
}
