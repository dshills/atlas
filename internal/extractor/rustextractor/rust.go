// Package rustextractor implements a regex/heuristic-based extractor for Rust.
package rustextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

// RustExtractor implements extractor.Extractor for Rust files.
type RustExtractor struct{}

// New creates a new RustExtractor.
func New() *RustExtractor {
	return &RustExtractor{}
}

func (r *RustExtractor) Language() string { return "rust" }

func (r *RustExtractor) Supports(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".rs"
}

func (r *RustExtractor) SupportedKinds() []string {
	return []string{"module", "function", "method", "struct", "enum", "trait", "type", "const", "var", "test", "macro"}
}

func (r *RustExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	// Module name from file path (lib.rs / mod.rs use parent dir name)
	moduleName := deriveModuleName(req.FilePath)

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          filepath.Base(strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))),
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "rust",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	// Comment filter for relationship extraction
	codeLines := commentfilter.LineFilter(content, "rust")

	// Extract routes
	routeRefs, routeArts := extractRoutes(content, lines, codeLines)
	result.References = append(result.References, routeRefs...)
	result.Artifacts = append(result.Artifacts, routeArts...)

	// Extract config access
	configRefs, configArts := extractConfigAccess(content, lines, codeLines)
	result.References = append(result.References, configRefs...)
	result.Artifacts = append(result.Artifacts, configArts...)

	// Extract test references
	result.References = append(result.References, extractTestReferences(result.Symbols, moduleName)...)

	return result, nil
}

func deriveModuleName(path string) string {
	base := filepath.Base(path)
	if base == "lib.rs" || base == "mod.rs" || base == "main.rs" {
		dir := filepath.Dir(path)
		if dir == "." || dir == "" {
			return strings.TrimSuffix(base, ".rs")
		}
		return strings.ReplaceAll(dir, string(filepath.Separator), "::")
	}
	name := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ReplaceAll(name, string(filepath.Separator), "::")
}

// Regex patterns for Rust extraction.
var (
	useRe        = regexp.MustCompile(`(?m)^use\s+([^;]+);`)
	fnDeclRe     = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+(\w+)`)
	structRe     = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?struct\s+(\w+)`)
	enumRe       = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?enum\s+(\w+)`)
	traitRe      = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?trait\s+(\w+)`)
	implRe       = regexp.MustCompile(`(?m)^impl(?:<[^>]*>)?\s+(?:(\w+)\s+for\s+)?(\w+)`)
	typeAliasRe  = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?type\s+(\w+)\s*[=<]`)
	constRe      = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?(?:const|static)\s+(\w+)\s*:`)
	modRe        = regexp.MustCompile(`(?m)^(?:pub(?:\((?:crate|super)\)\s+|\s+))?mod\s+(\w+)`)
	macroRe      = regexp.MustCompile(`(?m)^macro_rules!\s+(\w+)`)
	implMethodRe = regexp.MustCompile(`(?m)^\s+(?:pub(?:\((?:crate|super)\)\s+|\s+))?(?:async\s+)?(?:unsafe\s+)?fn\s+(\w+)`)
)

func extractImports(content string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	matches := useRe.FindAllStringSubmatchIndex(content, -1)
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

	// Top-level functions
	fnMatches := fnDeclRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range fnMatches {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		// Skip indented fn declarations (methods inside impl blocks)
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}

		kind := "function"
		// Check if preceded by #[test]
		if isTestFn(lines, line-1) {
			kind = "test"
		}

		vis := visibility(lineContent)
		qname := moduleName + "::" + name
		stableID := "rust:" + qname + ":" + kind

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    kind,
			Visibility:    vis,
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      stableID,
		})
	}

	// Structs
	for _, m := range structRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "struct",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      "rust:" + qname + ":struct",
		})
	}

	// Enums
	for _, m := range enumRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "enum",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      "rust:" + qname + ":enum",
		})
	}

	// Traits
	for _, m := range traitRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "trait",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      "rust:" + qname + ":trait",
		})
	}

	// Impl blocks — extract methods
	implMatches := implRe.FindAllStringSubmatchIndex(content, -1)
	for _, im := range implMatches {
		typeName := content[im[4]:im[5]]
		implLine := strings.Count(content[:im[0]], "\n") + 1
		implEnd := findBlockEnd(lines, implLine-1)

		if implLine < len(lines) && implEnd <= len(lines) {
			for i := implLine; i < min(implEnd, len(lines)); i++ {
				mm := implMethodRe.FindStringSubmatch(lines[i])
				if mm == nil {
					continue
				}
				methodName := mm[1]
				qname := moduleName + "::" + typeName + "::" + methodName
				stableID := "rust:" + qname + ":method"

				mVis := "exported"
				trimmed := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(trimmed, "pub") {
					mVis = "unexported"
				}

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           methodName,
					QualifiedName:  qname,
					SymbolKind:     "method",
					Visibility:     mVis,
					ParentSymbolID: moduleName + "::" + typeName,
					StartLine:      i + 1,
					EndLine:        findBlockEnd(lines, i),
					StableID:       stableID,
				})
			}
		}
	}

	// Type aliases
	for _, m := range typeAliasRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "type",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       line,
			StableID:      "rust:" + qname + ":type",
		})
	}

	// Constants and statics
	for _, m := range constRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "const",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       line,
			StableID:      "rust:" + qname + ":const",
		})
	}

	// Modules
	for _, m := range modRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "module",
			Visibility:    "exported",
			StartLine:     line,
			StableID:      "rust:" + qname + ":module",
		})
	}

	// Macros
	for _, m := range macroRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + "::" + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "macro",
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      "rust:" + qname + ":macro",
		})
	}

	// Test functions (at any indentation, preceded by #[test])
	symbols = append(symbols, extractTestFns(lines, moduleName)...)

	return symbols
}

// extractTestFns finds all #[test]-annotated functions at any indentation level,
// which the top-level fnDeclRe would skip because they're indented inside mod blocks.
func extractTestFns(lines []string, moduleName string) []extractor.SymbolRecord {
	var symbols []extractor.SymbolRecord
	testFnRe := regexp.MustCompile(`^\s+(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "#[test]" && trimmed != "#[tokio::test]" {
			continue
		}
		// Look forward for the fn declaration
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			jTrimmed := strings.TrimSpace(lines[j])
			if jTrimmed == "" || strings.HasPrefix(jTrimmed, "#[") {
				continue
			}
			m := testFnRe.FindStringSubmatch(lines[j])
			if m != nil {
				name := m[1]
				qname := moduleName + "::" + name
				stableID := "rust:" + qname + ":test"
				symbols = append(symbols, extractor.SymbolRecord{
					Name:          name,
					QualifiedName: qname,
					SymbolKind:    "test",
					Visibility:    "unexported",
					StartLine:     j + 1,
					EndLine:       findBlockEnd(lines, j),
					StableID:      stableID,
				})
			}
			break
		}
	}
	return symbols
}

func visibility(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "pub") {
		return "exported"
	}
	return "unexported"
}

func isTestFn(lines []string, fnLineIdx int) bool {
	// Look backwards from the fn line for #[test] attribute
	for i := fnLineIdx - 1; i >= 0 && i >= fnLineIdx-3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "#[test]" || trimmed == "#[tokio::test]" {
			return true
		}
		// Stop if we hit a non-attribute, non-blank line
		if trimmed != "" && !strings.HasPrefix(trimmed, "#[") {
			break
		}
	}
	return false
}

// findBlockEnd finds the closing brace for a block starting at the given line index.
func findBlockEnd(lines []string, startIdx int) int {
	depth := 0
	for i := startIdx; i < len(lines); i++ {
		for _, ch := range lines[i] {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
	}
	return startIdx + 1
}
