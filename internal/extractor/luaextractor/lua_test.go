package luaextractor

import (
	"context"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestLuaExtractor_Language(t *testing.T) {
	e := New()
	if got := e.Language(); got != "lua" {
		t.Errorf("Language() = %q, want %q", got, "lua")
	}
}

func TestLuaExtractor_Supports(t *testing.T) {
	e := New()
	tests := []struct {
		path string
		want bool
	}{
		{"init.lua", true},
		{"src/app.lua", true},
		{"main.go", false},
		{"script.LUA", true},
	}
	for _, tt := range tests {
		if got := e.Supports(tt.path); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestLuaExtractor_Extract_BasicFunction(t *testing.T) {
	e := New()
	content := `local json = require("cjson")

function greet(name)
    print("Hello " .. name)
end

local function helper()
    return 42
end
`
	result, err := e.Extract(context.Background(), extractor.ExtractRequest{
		FilePath: "app.lua",
		Content:  []byte(content),
	})
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}
	if result.File.ParseStatus != "ok" {
		t.Errorf("ParseStatus = %q, want %q", result.File.ParseStatus, "ok")
	}
	if result.Package.Language != "lua" {
		t.Errorf("Language = %q, want %q", result.Package.Language, "lua")
	}

	// Should have at least 1 import reference
	hasImport := false
	for _, r := range result.References {
		if r.ReferenceKind == "imports" && r.RawTargetText == "cjson" {
			hasImport = true
		}
	}
	if !hasImport {
		t.Error("expected import reference for cjson")
	}

	// Should have symbols for greet and helper
	symbolNames := map[string]bool{}
	for _, s := range result.Symbols {
		symbolNames[s.Name] = true
	}
	if !symbolNames["greet"] {
		t.Error("expected symbol 'greet'")
	}
	if !symbolNames["helper"] {
		t.Error("expected symbol 'helper'")
	}
}

func TestLuaExtractor_Extract_Methods(t *testing.T) {
	e := New()
	content := `function MyClass.new(self)
    return setmetatable({}, self)
end

function MyClass:greet()
    print("hello")
end
`
	result, err := e.Extract(context.Background(), extractor.ExtractRequest{
		FilePath: "myclass.lua",
		Content:  []byte(content),
	})
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	methods := 0
	for _, s := range result.Symbols {
		if s.SymbolKind == "method" {
			methods++
		}
	}
	if methods != 2 {
		t.Errorf("expected 2 methods, got %d", methods)
	}
}

func TestDeriveModuleName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"app.lua", "app"},
		{"src/utils.lua", "src.utils"},
		{"init.lua", "init"},
		{"lib/init.lua", "lib"},
	}
	for _, tt := range tests {
		if got := deriveModuleName(tt.path); got != tt.want {
			t.Errorf("deriveModuleName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
