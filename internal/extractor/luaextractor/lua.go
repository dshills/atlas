// Package luaextractor implements a regex/heuristic-based extractor for Lua.
package luaextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

// LuaExtractor implements extractor.Extractor for Lua files.
type LuaExtractor struct{}

// New creates a new LuaExtractor.
func New() *LuaExtractor {
	return &LuaExtractor{}
}

func (l *LuaExtractor) Language() string { return "lua" }

func (l *LuaExtractor) Supports(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".lua"
}

func (l *LuaExtractor) SupportedKinds() []string {
	return []string{"module", "function", "method", "var", "const", "test"}
}

func (l *LuaExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	// Module name from file path
	moduleName := deriveModuleName(req.FilePath)

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          filepath.Base(strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))),
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "lua",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	// Comment filter for relationship extraction
	codeLines := commentfilter.LineFilter(content, "lua")

	// Extract routes
	routeRefs, routeArts := extractRoutes(content, lines, codeLines)
	result.References = append(result.References, routeRefs...)
	result.Artifacts = append(result.Artifacts, routeArts...)

	// Extract config access
	configRefs, configArts := extractConfigAccess(content, lines, codeLines)
	result.References = append(result.References, configRefs...)
	result.Artifacts = append(result.Artifacts, configArts...)

	// Extract SQL artifacts
	sqlRefs, sqlArts := extractSQLArtifacts(content, lines, req.FilePath, codeLines)
	result.References = append(result.References, sqlRefs...)
	result.Artifacts = append(result.Artifacts, sqlArts...)

	// Extract services
	svcRefs, svcArts := extractServices(content, lines, codeLines)
	result.References = append(result.References, svcRefs...)
	result.Artifacts = append(result.Artifacts, svcArts...)

	// Extract calls
	result.References = append(result.References, extractCalls(content, lines, codeLines, result.Symbols, moduleName)...)

	// Extract test references
	result.References = append(result.References, extractTestReferences(result.Symbols, moduleName)...)

	return result, nil
}

func deriveModuleName(path string) string {
	// init.lua uses parent directory name, similar to Rust's mod.rs
	base := filepath.Base(path)
	if base == "init.lua" {
		dir := filepath.Dir(path)
		if dir == "." || dir == "" {
			return "init"
		}
		return strings.ReplaceAll(dir, string(filepath.Separator), ".")
	}
	name := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ReplaceAll(name, string(filepath.Separator), ".")
}

// Regex patterns for Lua extraction.
var (
	requireRe    = regexp.MustCompile(`(?m)require\s*[\("]\s*['"]?([^'")\s]+)['"]?\s*\)?`)
	globalFuncRe = regexp.MustCompile(`(?m)^function\s+(\w+)\s*\(`)
	localFuncRe  = regexp.MustCompile(`(?m)^local\s+function\s+(\w+)\s*\(`)
	methodDeclRe = regexp.MustCompile(`(?m)^function\s+(\w+)[.:](\w+)\s*\(`)
	localVarRe   = regexp.MustCompile(`(?m)^local\s+([A-Z_][A-Z0-9_]*)\s*=`)
	bustedDescRe = regexp.MustCompile(`(?m)^\s*describe\s*\(\s*["']`)
	bustedItRe   = regexp.MustCompile(`(?m)^\s*it\s*\(\s*["']([^"']+)["']`)
)

func extractImports(content string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	matches := requireRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		importPath := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		refs = append(refs, extractor.ReferenceRecord{
			ToSymbolName:  importPath,
			ReferenceKind: "imports",
			Confidence:    "exact",
			Line:          line,
			RawTargetText: importPath,
		})
	}
	return refs
}

func extractSymbols(content string, lines []string, moduleName string) []extractor.SymbolRecord {
	var symbols []extractor.SymbolRecord

	isTestFile := isTestFilePath(filepath.Base(moduleName))

	// Global functions: function name(...)
	for _, m := range globalFuncRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		// Skip if this is actually a method declaration (name.method or name:method)
		if methodDeclRe.MatchString(lines[line-1]) {
			continue
		}

		kind := "function"
		if isTestFile && (strings.HasPrefix(name, "test") || strings.HasPrefix(name, "Test")) {
			kind = "test"
		}

		qname := moduleName + "." + name
		stableID := "lua:" + qname + ":" + kind

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    kind,
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      stableID,
		})
	}

	// Local functions: local function name(...)
	for _, m := range localFuncRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		kind := "function"
		if isTestFile && (strings.HasPrefix(name, "test") || strings.HasPrefix(name, "Test")) {
			kind = "test"
		}

		qname := moduleName + "." + name
		stableID := "lua:" + qname + ":" + kind

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    kind,
			Visibility:    "unexported",
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      stableID,
		})
	}

	// Method declarations: function Obj.method(...) or function Obj:method(...)
	for _, m := range methodDeclRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		methodName := content[m[4]:m[5]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + "." + receiver + "." + methodName
		stableID := "lua:" + qname + ":method"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:           methodName,
			QualifiedName:  qname,
			SymbolKind:     "method",
			Visibility:     "exported",
			ParentSymbolID: moduleName + "." + receiver,
			StartLine:      line,
			EndLine:        findBlockEnd(lines, line-1),
			StableID:       stableID,
		})
	}

	// Local constants (ALL_CAPS)
	for _, m := range localVarRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + "." + name
		stableID := "lua:" + qname + ":const"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "const",
			Visibility:    "unexported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	// Busted test framework: it("description", function() ...)
	if bustedDescRe.MatchString(content) {
		for _, m := range bustedItRe.FindAllStringSubmatchIndex(content, -1) {
			desc := content[m[2]:m[3]]
			line := strings.Count(content[:m[0]], "\n") + 1

			// Use description as name, sanitized
			name := sanitizeTestName(desc)
			qname := moduleName + "." + name
			stableID := "lua:" + qname + ":test"

			symbols = append(symbols, extractor.SymbolRecord{
				Name:          name,
				QualifiedName: qname,
				SymbolKind:    "test",
				Visibility:    "unexported",
				StartLine:     line,
				EndLine:       findBlockEnd(lines, line-1),
				StableID:      stableID,
			})
		}
	}

	return symbols
}

func isTestFilePath(base string) bool {
	lower := strings.ToLower(base)
	return strings.HasPrefix(lower, "test") || strings.HasSuffix(lower, "_test") ||
		strings.HasSuffix(lower, "_spec") || strings.Contains(lower, "test")
}

func sanitizeTestName(desc string) string {
	// Replace spaces and special chars with underscores
	var b strings.Builder
	for _, r := range desc {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('_')
		}
	}
	result := b.String()
	if len(result) > 60 {
		result = result[:60]
	}
	return result
}

// findBlockEnd finds the matching 'end' keyword for a block starting at the given line index.
// Lua uses keyword-based blocks (function/if/for/while/repeat...end).
func findBlockEnd(lines []string, startIdx int) int {
	depth := 0
	for i := startIdx; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Count block openers
		depth += countBlockOpeners(trimmed)

		// Count block closers
		if trimmed == "end" || strings.HasPrefix(trimmed, "end ") ||
			strings.HasPrefix(trimmed, "end)") || strings.HasPrefix(trimmed, "end,") ||
			strings.HasSuffix(trimmed, " end") || strings.HasSuffix(trimmed, ")end") {
			depth--
			if depth <= 0 {
				return i + 1
			}
		}
	}
	return startIdx + 1
}

// countBlockOpeners counts how many block-opening keywords appear on a line.
func countBlockOpeners(trimmed string) int {
	count := 0
	openers := []string{"function", "if ", "for ", "while ", "repeat"}
	for _, op := range openers {
		if strings.HasPrefix(trimmed, op) || strings.Contains(trimmed, " "+op) ||
			strings.Contains(trimmed, "("+op) || strings.Contains(trimmed, "="+op) {
			count++
		}
	}
	return count
}
