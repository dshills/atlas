// Package tsextractor implements a regex/heuristic-based extractor for TypeScript and JavaScript.
package tsextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

// TSExtractor implements extractor.Extractor for TypeScript and JavaScript files.
type TSExtractor struct{}

// New creates a new TSExtractor.
func New() *TSExtractor {
	return &TSExtractor{}
}

func (t *TSExtractor) Language() string { return "typescript" }

func (t *TSExtractor) Supports(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx"
}

func (t *TSExtractor) SupportedKinds() []string {
	return []string{"module", "function", "method", "class", "interface", "type", "const", "var", "test", "entrypoint"}
}

func (t *TSExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	ext := strings.ToLower(filepath.Ext(req.FilePath))
	language := "typescript"
	if ext == ".js" || ext == ".jsx" {
		language = "javascript"
	}

	// Module name from file path (relative path without extension)
	moduleName := strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))
	moduleName = strings.ReplaceAll(moduleName, string(filepath.Separator), "/")

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          filepath.Base(strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))),
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      language,
		},
	}

	isTestFile := isTestFilePath(req.FilePath)

	// Extract imports
	result.References = append(result.References, extractImports(content, lines)...)

	// Extract symbols
	symbols := extractSymbols(content, lines, moduleName, language)
	result.Symbols = append(result.Symbols, symbols...)

	// Extract test calls
	if isTestFile {
		result.Symbols = append(result.Symbols, extractTestCalls(content, lines, moduleName, language)...)
	}

	// Comment filter for relationship extraction
	codeLines := commentfilter.LineFilter(content, language)

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

func isTestFilePath(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
}

// Regex patterns for extraction
var (
	importRe        = regexp.MustCompile(`(?m)^import\s+(?:(?:(?:\{[^}]*\}|\*\s+as\s+\w+|\w+)(?:\s*,\s*(?:\{[^}]*\}|\*\s+as\s+\w+))?)\s+from\s+)?['"]([^'"]+)['"]`)
	funcDeclRe      = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	arrowFuncRe     = regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[a-zA-Z_]\w*)\s*(?::\s*[^=]+)?\s*=>`)
	classDeclRe     = regexp.MustCompile(`(?m)^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	methodRe        = regexp.MustCompile(`(?m)^\s+(?:static\s+)?(?:async\s+)?(?:get\s+|set\s+)?(\w+)\s*\(`)
	interfaceDeclRe = regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`)
	typeDeclRe      = regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)\s*[=<]`)
	constDeclRe     = regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*(?::\s*[^=]+)?\s*=`)
	varDeclRe       = regexp.MustCompile(`(?m)^(?:export\s+)?(?:let|var)\s+(\w+)`)
	testCallRe      = regexp.MustCompile(`(?m)(?:describe|it|test)\s*\(\s*['"]([^'"]+)['"]`)
)

func extractImports(content string, _ []string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	matches := importRe.FindAllStringSubmatchIndex(content, -1)
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

func extractSymbols(content string, lines []string, moduleName, language string) []extractor.SymbolRecord {
	var symbols []extractor.SymbolRecord

	type symMatch struct {
		re   *regexp.Regexp
		kind string
	}
	matchers := []symMatch{
		{funcDeclRe, "function"},
		{arrowFuncRe, "function"},
		{classDeclRe, "class"},
		{interfaceDeclRe, "interface"},
		{typeDeclRe, "type"},
		{constDeclRe, "const"},
		{varDeclRe, "var"},
	}

	for _, sm := range matchers {
		matches := sm.re.FindAllStringSubmatchIndex(content, -1)
		for _, m := range matches {
			name := content[m[2]:m[3]]
			line := strings.Count(content[:m[0]], "\n") + 1
			lineContent := ""
			if line-1 < len(lines) {
				lineContent = lines[line-1]
			}

			vis := "unexported"
			if strings.Contains(lineContent, "export ") {
				vis = "exported"
			}

			kind := sm.kind

			qname := moduleName + "." + name
			stableID := language + ":" + qname + ":" + kind

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
	}

	// Extract methods within classes
	classMatches := classDeclRe.FindAllStringSubmatchIndex(content, -1)
	for _, cm := range classMatches {
		className := content[cm[2]:cm[3]]
		classLine := strings.Count(content[:cm[0]], "\n") + 1
		classEnd := findBlockEnd(lines, classLine-1)

		// Find methods within the class body
		classBody := ""
		if classLine-1 < len(lines) && classEnd <= len(lines) {
			classBody = strings.Join(lines[classLine:min(classEnd, len(lines))], "\n")
		}

		methodMatches := methodRe.FindAllStringSubmatch(classBody, -1)
		for _, mm := range methodMatches {
			methodName := mm[1]
			if methodName == "constructor" || methodName == "if" || methodName == "for" || methodName == "while" || methodName == "switch" {
				continue
			}

			qname := moduleName + "." + className + "." + methodName
			stableID := language + ":" + qname + ":method"

			symbols = append(symbols, extractor.SymbolRecord{
				Name:           methodName,
				QualifiedName:  qname,
				SymbolKind:     "method",
				Visibility:     "exported",
				ParentSymbolID: moduleName + "." + className,
				StableID:       stableID,
			})
		}
	}

	return symbols
}

func extractTestCalls(content string, _ []string, moduleName, language string) []extractor.SymbolRecord {
	var symbols []extractor.SymbolRecord
	matches := testCallRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		// Use a sanitized name for qualified name
		safeName := strings.ReplaceAll(name, " ", "_")
		qname := moduleName + "." + safeName
		stableID := language + ":" + qname + ":test"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "test",
			Visibility:    "unexported",
			StartLine:     line,
			StableID:      stableID,
		})
	}
	return symbols
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
					return i + 1 // 1-indexed
				}
			}
		}
	}
	return startIdx + 1
}
