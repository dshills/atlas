package luaextractor

import (
	"context"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
)

func TestLuaLanguage(t *testing.T) {
	ext := New()
	if got := ext.Language(); got != "lua" {
		t.Errorf("Language() = %q, want %q", got, "lua")
	}
}

func TestLuaSupports(t *testing.T) {
	ext := New()
	tests := []struct {
		path string
		want bool
	}{
		{"init.lua", true},
		{"src/utils/helpers.lua", true},
		{"test.LUA", true},
		{"main.go", false},
		{"script.py", false},
		{"lua", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := ext.Supports(tt.path); got != tt.want {
				t.Errorf("Supports(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractLuaBasic(t *testing.T) {
	ext := New()
	content := `local socket = require("socket")
local json = require "cjson"
require("utils.helpers")

local M = {}

local count = 0
MAX_SIZE = 100

local function helper(x)
    return x + 1
end

function M.greet(name)
    return "Hello, " .. name
end

local add = function(a, b)
    return a + b
end

function Player:move(dx, dy)
    self.x = self.x + dx
    self.y = self.y + dy
end

return M
`
	req := extractor.ExtractRequest{
		FilePath: "src/utils/helpers.lua",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Check package
	if result.Package.Name != "helpers" {
		t.Errorf("expected package name 'helpers', got %q", result.Package.Name)
	}
	if result.Package.ImportPath != "src.utils.helpers" {
		t.Errorf("expected import path 'src.utils.helpers', got %q", result.Package.ImportPath)
	}
	if result.Package.Language != "lua" {
		t.Errorf("expected language 'lua', got %q", result.Package.Language)
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

	// Check symbol kinds
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	// Methods: Player:move
	if kinds["method"] != 1 {
		t.Errorf("expected 1 method, got %d", kinds["method"])
	}
	// Functions: helper, M.greet, add
	if kinds["function"] != 3 {
		t.Errorf("expected 3 functions, got %d", kinds["function"])
	}
	// Vars: M, count, MAX_SIZE
	if kinds["var"] != 3 {
		t.Errorf("expected 3 vars, got %d", kinds["var"])
	}
}

func TestExtractLuaTestDetection(t *testing.T) {
	ext := New()
	content := `local function testAddition()
    assert(1 + 1 == 2)
end

function testSubtraction()
    assert(2 - 1 == 1)
end

describe("Calculator", function()
    it("should add numbers", function()
        assert.equal(2, 1 + 1)
    end)

    it("should subtract numbers", function()
        assert.equal(1, 2 - 1)
    end)
end)
`
	req := extractor.ExtractRequest{
		FilePath: "test/calc_test.lua",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	testCount := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "test" {
			testCount++
		}
	}
	// testAddition, testSubtraction, describe("Calculator"), it("should add..."), it("should subtract...")
	if testCount != 5 {
		t.Errorf("expected 5 test symbols, got %d", testCount)
		for _, sym := range result.Symbols {
			if sym.SymbolKind == "test" {
				t.Logf("  test: %s (%s)", sym.Name, sym.QualifiedName)
			}
		}
	}
}

func TestExtractLuaVisibility(t *testing.T) {
	ext := New()
	content := `local function privateHelper()
    return true
end

function publicFunc()
    return false
end

local secret = 42
visible = 99
`
	req := extractor.ExtractRequest{
		FilePath: "mod.lua",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	symVis := make(map[string]string)
	for _, sym := range result.Symbols {
		symVis[sym.Name] = sym.Visibility
	}

	tests := []struct {
		name    string
		wantVis string
	}{
		{"privateHelper", "unexported"},
		{"publicFunc", "exported"},
		{"secret", "unexported"},
		{"visible", "exported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vis, ok := symVis[tt.name]
			if !ok {
				t.Fatalf("symbol %q not found", tt.name)
			}
			if vis != tt.wantVis {
				t.Errorf("symbol %q visibility = %q, want %q", tt.name, vis, tt.wantVis)
			}
		})
	}
}

func TestExtractLuaQualifiedNames(t *testing.T) {
	ext := New()
	content := `function M.helper(x)
    return x
end

function Player:move(dx, dy)
    self.x = self.x + dx
end
`
	req := extractor.ExtractRequest{
		FilePath: "game/player.lua",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	qnames := make(map[string]string)
	parents := make(map[string]string)
	for _, sym := range result.Symbols {
		qnames[sym.Name] = sym.QualifiedName
		parents[sym.Name] = sym.ParentSymbolID
	}

	// M.helper -> qualified name includes module
	if got := qnames["helper"]; got != "game.player.M.helper" {
		t.Errorf("helper qualified name = %q, want %q", got, "game.player.M.helper")
	}
	if got := parents["helper"]; got != "game.player.M" {
		t.Errorf("helper parent = %q, want %q", got, "game.player.M")
	}

	// Player:move -> Class:method notation
	if got := qnames["move"]; got != "game.player.Player:move" {
		t.Errorf("move qualified name = %q, want %q", got, "game.player.Player:move")
	}
	if got := parents["move"]; got != "game.player.Player" {
		t.Errorf("move parent = %q, want %q", got, "game.player.Player")
	}
}

func TestExtractLuaBlockEnd(t *testing.T) {
	lines := []string{
		"function foo()",        // 0: depth +1 (function)
		"    if x > 0 then",     // 1: depth +1 (if)
		"        for i=1,10 do", // 2: depth +1 (for)
		"            print(i)",  // 3
		"        end",           // 4: depth -1
		"    end",               // 5: depth -1
		"end",                   // 6: depth -1 -> 0
		"print('after')",        // 7
	}

	endLine := findBlockEnd(lines, 0)
	// Line 6 (0-indexed) = line 7 (1-indexed)
	if endLine != 7 {
		t.Errorf("findBlockEnd returned %d, want 7", endLine)
	}
}

func TestExtractLuaBlockEnd_RepeatUntil(t *testing.T) {
	lines := []string{
		"repeat",        // 0: depth +1 (repeat)
		"    x = x + 1", // 1
		"until x > 10",  // 2: depth -1 -> 0
		"print('done')", // 3
	}

	endLine := findBlockEnd(lines, 0)
	if endLine != 3 {
		t.Errorf("findBlockEnd for repeat..until returned %d, want 3", endLine)
	}
}

func TestExtractLuaBlockEnd_WhileDo(t *testing.T) {
	// "do" on the same line as "while" should NOT increment depth extra.
	// "while" itself opens the block.
	lines := []string{
		"while x > 0 do", // 0: depth +1 (while), "do" is not bare
		"    x = x - 1",  // 1
		"end",            // 2: depth -1 -> 0
	}

	endLine := findBlockEnd(lines, 0)
	if endLine != 3 {
		t.Errorf("findBlockEnd for while..do..end returned %d, want 3", endLine)
	}
}

func TestExtractLuaBlockEnd_BareDo(t *testing.T) {
	lines := []string{
		"do",             // 0: bare do, depth +1
		"    print('x')", // 1
		"end",            // 2: depth -1 -> 0
	}

	endLine := findBlockEnd(lines, 0)
	if endLine != 3 {
		t.Errorf("findBlockEnd for bare do..end returned %d, want 3", endLine)
	}
}

func TestExtractLuaBlockEnd_NestedFunctions(t *testing.T) {
	lines := []string{
		"function outer()",           // 0: depth +1
		"    local function inner()", // 1: depth +1
		"        return 42",          // 2
		"    end",                    // 3: depth -1
		"    return inner",           // 4
		"end",                        // 5: depth -1 -> 0
	}

	endLine := findBlockEnd(lines, 0)
	if endLine != 6 {
		t.Errorf("findBlockEnd for nested functions returned %d, want 6", endLine)
	}
}

func TestExtract_FullPipeline(t *testing.T) {
	ext := New()
	content := `-- A full Lua module
local http = require("socket.http")
local json = require("cjson")

local M = {}

local VERSION = "1.0"

function M.init(config)
    if config.debug then
        print("debug mode")
    end
    return true
end

local function validate(input)
    for _, v in ipairs(input) do
        if not v then
            return false
        end
    end
    return true
end

function Player:update(dt)
    self.x = self.x + self.vx * dt
end

M.process = function(data)
    return data
end

function testInit()
    assert(M.init({debug = true}))
end

describe("Player", function()
    it("should update position", function()
        local p = Player:new()
        p:update(1.0)
    end)
end)

return M
`

	req := extractor.ExtractRequest{
		FilePath: "lib/game/engine.lua",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify file record
	if result.File.ParseStatus != "ok" {
		t.Errorf("ParseStatus = %q, want %q", result.File.ParseStatus, "ok")
	}

	// Verify package
	if result.Package.Name != "engine" {
		t.Errorf("package name = %q, want %q", result.Package.Name, "engine")
	}
	if result.Package.ImportPath != "lib.game.engine" {
		t.Errorf("import path = %q, want %q", result.Package.ImportPath, "lib.game.engine")
	}

	// Verify imports
	imports := 0
	for _, ref := range result.References {
		if ref.ReferenceKind == "imports" {
			imports++
			if ref.Confidence != "exact" {
				t.Errorf("import confidence = %q, want %q", ref.Confidence, "exact")
			}
		}
	}
	if imports != 2 {
		t.Errorf("expected 2 imports, got %d", imports)
	}

	// Verify symbol counts by kind
	kinds := make(map[string]int)
	for _, sym := range result.Symbols {
		kinds[sym.SymbolKind]++
	}

	// functions: M.init, validate, process
	if kinds["function"] != 3 {
		t.Errorf("expected 3 functions, got %d", kinds["function"])
	}
	// methods: Player:update
	if kinds["method"] != 1 {
		t.Errorf("expected 1 method, got %d", kinds["method"])
	}
	// vars: M, VERSION
	if kinds["var"] != 2 {
		t.Errorf("expected 2 vars, got %d", kinds["var"])
	}
	// tests: testInit, describe("Player"), it("should update position")
	if kinds["test"] != 3 {
		t.Errorf("expected 3 test symbols, got %d", kinds["test"])
		for _, sym := range result.Symbols {
			t.Logf("  sym: %s kind=%s", sym.Name, sym.SymbolKind)
		}
	}

	// Verify stable IDs contain "lua:" prefix
	for _, sym := range result.Symbols {
		if !hasPrefix(sym.StableID, "lua:") {
			t.Errorf("symbol %q stable ID = %q, expected lua: prefix", sym.Name, sym.StableID)
		}
	}
}

func TestLuaMethodExtraction(t *testing.T) {
	ext := New()
	content := `function Foo:bar()
    return self.x
end

function Foo:baz(a, b)
    return a + b
end
`
	req := extractor.ExtractRequest{
		FilePath: "mymod.lua",
		Content:  []byte(content),
	}

	result, err := ext.Extract(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	methods := 0
	for _, sym := range result.Symbols {
		if sym.SymbolKind == "method" {
			methods++
			if sym.ParentSymbolID != "mymod.Foo" {
				t.Errorf("method %q parent = %q, want %q", sym.Name, sym.ParentSymbolID, "mymod.Foo")
			}
			if sym.Visibility != "exported" {
				t.Errorf("method %q visibility = %q, want %q", sym.Name, sym.Visibility, "exported")
			}
			// Check StableID format
			expectedPrefix := "lua:mymod.Foo:" + sym.Name + ":method"
			if sym.StableID != expectedPrefix {
				t.Errorf("method %q StableID = %q, want %q", sym.Name, sym.StableID, expectedPrefix)
			}
		}
	}

	if methods != 2 {
		t.Errorf("expected 2 methods, got %d", methods)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
