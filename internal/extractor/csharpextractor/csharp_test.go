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
using System.Linq;

public class UserService
{
    public const int MAX_USERS = 100;

    public void CreateUser(string name)
    {
        // implementation
    }

    private bool ValidateEmail(string email)
    {
        return email.Contains("@");
    }
}

public interface IUserRepository
{
    void Save(User user);
    User FindById(string id);
}

public enum UserStatus
{
    Active,
    Inactive,
    Suspended
}

public struct UserInfo
{
    public string Name;
    public int Age;
}
`

	req := extractor.ExtractRequest{
		FilePath: "src/Services/UserService.cs",
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

	// Check imports
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
	if kinds["method"] < 2 {
		t.Errorf("expected at least 2 methods, got %d", kinds["method"])
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

public class UserServiceTest
{
    [Test]
    public void ShouldCreateUser()
    {
        // test code
    }

    [Fact]
    public void ShouldValidateEmail()
    {
        // test code
    }

    public void HelperMethod()
    {
        // not a test
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "tests/UserServiceTest.cs",
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

	if testCount != 2 {
		t.Errorf("expected 2 test symbols, got %d", testCount)
		for _, sym := range result.Symbols {
			t.Logf("  symbol: %s kind=%s line=%d", sym.Name, sym.SymbolKind, sym.StartLine)
		}
	}
	if methodCount != 1 {
		t.Errorf("expected 1 non-test method, got %d", methodCount)
	}
}

func TestCSharpSupports(t *testing.T) {
	ext := New()
	cases := map[string]bool{
		"File.cs":     true,
		"Main.cs":     true,
		"Test.CS":     true,
		"file.go":     false,
		"file.java":   false,
		"file.ts":     false,
		"file.csproj": false,
		"file.cs.bak": false,
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
		"struct": true, "method": true, "field": true, "const": true, "test": true,
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
	content := `public class Simple
{
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

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `namespace MyApp.Controllers;

using System;
using Microsoft.AspNetCore.Mvc;

[ApiController]
[Route("api/[controller]")]
public class UsersController : ControllerBase
{
    [HttpGet("users/{id}")]
    public IActionResult GetUser(int id)
    {
        var connectionString = Environment.GetEnvironmentVariable("DB_CONNECTION");
        var query = "SELECT * FROM Users WHERE Id = @id";
        return Ok();
    }

    [HttpPost("users")]
    public IActionResult CreateUser()
    {
        return Created();
    }
}
`

	req := extractor.ExtractRequest{
		FilePath: "Controllers/UsersController.cs",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Should have routes
	routeCount := 0
	for _, r := range result.References {
		if r.ReferenceKind == "registers_route" {
			routeCount++
		}
	}
	if routeCount < 2 {
		t.Errorf("expected at least 2 route refs, got %d", routeCount)
	}

	// Should have config access
	configCount := 0
	for _, r := range result.References {
		if r.ReferenceKind == "uses_config" {
			configCount++
		}
	}
	if configCount < 1 {
		t.Errorf("expected at least 1 config ref, got %d", configCount)
	}

	// Should have SQL
	sqlCount := 0
	for _, r := range result.References {
		if r.ReferenceKind == "touches_table" {
			sqlCount++
		}
	}
	if sqlCount < 1 {
		t.Errorf("expected at least 1 SQL ref, got %d", sqlCount)
	}

	// Should have calls
	callCount := 0
	for _, r := range result.References {
		if r.ReferenceKind == "calls" {
			callCount++
		}
	}
	if callCount < 1 {
		t.Errorf("expected at least 1 call ref, got %d", callCount)
	}
}
