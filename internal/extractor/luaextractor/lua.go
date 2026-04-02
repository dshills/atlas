// Package luaextractor implements a regex/heuristic-based extractor for Lua.
package luaextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
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

	// Module name derived from file path: replace separators with dots, strip extension.
	moduleName := deriveModuleName(req.FilePath)

	// Package name is the file base name without extension.
	pkgName := strings.TrimSuffix(filepath.Base(req.FilePath), filepath.Ext(req.FilePath))

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          pkgName,
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "lua",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	return result, nil
}

// deriveModuleName converts a file path to a Lua module name using dot separators.
// E.g., "src/utils/helpers.lua" -> "src.utils.helpers"
func deriveModuleName(filePath string) string {
	name := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	// Normalize separators to dots.
	name = strings.ReplaceAll(name, string(filepath.Separator), ".")
	// Also handle forward slashes in case paths use them on any OS.
	name = strings.ReplaceAll(name, "/", ".")
	return name
}

// Regex patterns for Lua extraction.
var (
	requireRe = regexp.MustCompile(`(?m)(?:local\s+\w+\s*=\s*)?require\s*\(?["']([^"']+)["']\)?`)

	funcDeclRe   = regexp.MustCompile(`(?m)^(?:local\s+)?function\s+(\w+(?:\.\w+)*)\s*\(`)
	funcAssignRe = regexp.MustCompile(`(?m)^(?:local\s+)?(\w+(?:\.\w+)*)\s*=\s*function\s*\(`)
	methodDeclRe = regexp.MustCompile(`(?m)^function\s+(\w+):(\w+)\s*\(`)
	localVarRe   = regexp.MustCompile(`(?m)^local\s+(\w+)\s*=`)
	moduleVarRe  = regexp.MustCompile(`(?m)^(\w+)\s*=\s*\S`)

	testFuncRe   = regexp.MustCompile(`(?m)^(?:local\s+)?function\s+(test\w+)\s*\(`)
	bustedDescRe = regexp.MustCompile(`(?m)describe\s*\(\s*['"]([^'"]+)['"]`)
	bustedItRe   = regexp.MustCompile(`(?m)it\s*\(\s*['"]([^'"]+)['"]`)

	// Block-opening keywords for findBlockEnd.
	blockOpenRe = regexp.MustCompile(`\b(function|if|for|while|repeat)\b`)
	// Bare "do" at the start of a line (optional whitespace before it).
	bareDoRe = regexp.MustCompile(`(?m)^\s*do\b`)
	// Block-closing keywords.
	blockEndRe   = regexp.MustCompile(`\bend\b`)
	blockUntilRe = regexp.MustCompile(`\buntil\b`)
)

func extractImports(content string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	matches := requireRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		importPath := strings.TrimSpace(content[m[2]:m[3]])
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

	// Track lines that have require calls to avoid counting as vars.
	requireLines := make(map[int]bool)
	for _, m := range requireRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		requireLines[line] = true
	}

	// Track lines that have function assignments to avoid double-counting as vars.
	funcAssignLines := make(map[int]bool)
	for _, m := range funcAssignRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		funcAssignLines[line] = true
	}

	// Track lines that are method declarations to avoid funcDeclRe matching them.
	methodDeclLines := make(map[int]bool)
	for _, m := range methodDeclRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		methodDeclLines[line] = true
	}

	// Track lines that have test functions to mark them as "test" kind.
	testFuncLines := make(map[int]bool)
	for _, m := range testFuncRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		testFuncLines[line] = true
	}

	// Track lines that have funcDeclRe to avoid double-counting as vars.
	funcDeclLines := make(map[int]bool)
	for _, m := range funcDeclRe.FindAllStringIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n") + 1
		funcDeclLines[line] = true
	}

	// Method declarations: function ClassName:methodName(
	for _, m := range methodDeclRe.FindAllStringSubmatchIndex(content, -1) {
		className := content[m[2]:m[3]]
		methodName := content[m[4]:m[5]]
		line := strings.Count(content[:m[0]], "\n") + 1
		endLine := findBlockEnd(lines, line-1)

		parentID := moduleName + "." + className
		qname := moduleName + "." + className + ":" + methodName
		stableID := "lua:" + qname + ":method"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:           methodName,
			QualifiedName:  qname,
			SymbolKind:     "method",
			Visibility:     "exported",
			ParentSymbolID: parentID,
			StartLine:      line,
			EndLine:        endLine,
			StableID:       stableID,
		})
	}

	// Named function declarations: [local] function name(
	for _, m := range funcDeclRe.FindAllStringSubmatchIndex(content, -1) {
		fullName := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		// Skip lines that are method declarations (matched by methodDeclRe).
		if methodDeclLines[line] {
			continue
		}

		endLine := findBlockEnd(lines, line-1)
		lineContent := lines[line-1]

		kind := "function"
		if testFuncLines[line] {
			kind = "test"
		}

		// Determine name and parent from dotted name.
		name := fullName
		parentID := ""
		if idx := strings.LastIndex(fullName, "."); idx >= 0 {
			name = fullName[idx+1:]
			parentID = moduleName + "." + fullName[:idx]
		}

		qname := moduleName + "." + fullName
		stableID := "lua:" + qname + ":" + kind
		vis := "exported"
		if strings.HasPrefix(strings.TrimSpace(lineContent), "local") {
			vis = "unexported"
		}

		symbols = append(symbols, extractor.SymbolRecord{
			Name:           name,
			QualifiedName:  qname,
			SymbolKind:     kind,
			Visibility:     vis,
			ParentSymbolID: parentID,
			StartLine:      line,
			EndLine:        endLine,
			StableID:       stableID,
		})
	}

	// Function assignments: [local] name = function(
	for _, m := range funcAssignRe.FindAllStringSubmatchIndex(content, -1) {
		fullName := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		endLine := findBlockEnd(lines, line-1)
		lineContent := lines[line-1]

		// Determine name and parent from dotted name.
		name := fullName
		parentID := ""
		if idx := strings.LastIndex(fullName, "."); idx >= 0 {
			name = fullName[idx+1:]
			parentID = moduleName + "." + fullName[:idx]
		}

		kind := "function"
		qname := moduleName + "." + fullName
		stableID := "lua:" + qname + ":" + kind
		vis := "exported"
		if strings.HasPrefix(strings.TrimSpace(lineContent), "local") {
			vis = "unexported"
		}

		symbols = append(symbols, extractor.SymbolRecord{
			Name:           name,
			QualifiedName:  qname,
			SymbolKind:     kind,
			Visibility:     vis,
			ParentSymbolID: parentID,
			StartLine:      line,
			EndLine:        endLine,
			StableID:       stableID,
		})
	}

	// Local variables: local name =
	for _, m := range localVarRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		// Skip if this line is a function assignment, function declaration, or require.
		if funcAssignLines[line] || funcDeclLines[line] || requireLines[line] {
			continue
		}

		qname := moduleName + "." + name
		stableID := "lua:" + qname + ":var"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "var",
			Visibility:    "unexported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	// Module-level variables: name = (not local, not function)
	seenModuleVars := make(map[string]bool)
	for _, m := range moduleVarRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := lines[line-1]

		// Skip if the line starts with "local".
		if strings.HasPrefix(strings.TrimSpace(lineContent), "local") {
			continue
		}
		// Skip if this line is a function assignment, function declaration, or require.
		if funcAssignLines[line] || funcDeclLines[line] || methodDeclLines[line] || requireLines[line] {
			continue
		}
		// Deduplicate: only emit the first assignment for each module-level var.
		if seenModuleVars[name] {
			continue
		}
		seenModuleVars[name] = true

		qname := moduleName + "." + name
		stableID := "lua:" + qname + ":var"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "var",
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	// Busted describe blocks.
	for _, m := range bustedDescRe.FindAllStringSubmatchIndex(content, -1) {
		desc := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + ".describe:" + desc
		stableID := "lua:" + qname + ":test"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          "describe: " + desc,
			QualifiedName: qname,
			SymbolKind:    "test",
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	// Busted it blocks.
	for _, m := range bustedItRe.FindAllStringSubmatchIndex(content, -1) {
		desc := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + ".it:" + desc
		stableID := "lua:" + qname + ":test"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          "it: " + desc,
			QualifiedName: qname,
			SymbolKind:    "test",
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	return symbols
}

// findBlockEnd finds the closing "end" (or "until") for a Lua block starting at
// the given line index (0-based). Lua uses keyword-based blocks instead of braces.
//
// Block-opening keywords: function, if, for, while, repeat
// "do" only counts when it appears at the start of a line (bare do...end block).
// Block-closing keywords: end, until (for repeat...until)
func findBlockEnd(lines []string, startIdx int) int {
	depth := 0
	for i := startIdx; i < len(lines); i++ {
		line := lines[i]

		// Count block-opening keywords on this line.
		opens := blockOpenRe.FindAllStringIndex(line, -1)
		depth += len(opens)

		// Count bare "do" at start of line.
		if bareDoRe.MatchString(line) {
			depth++
		}

		// Count block-closing "end" keywords.
		ends := blockEndRe.FindAllStringIndex(line, -1)
		depth -= len(ends)

		// Count "until" keywords (closes repeat blocks).
		untils := blockUntilRe.FindAllStringIndex(line, -1)
		depth -= len(untils)

		if depth <= 0 {
			return i + 1 // 1-indexed
		}
	}

	// If we never found the closing keyword, return startIdx + 1 as a fallback.
	return startIdx + 1
}
