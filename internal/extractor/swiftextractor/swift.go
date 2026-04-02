// Package swiftextractor implements a regex/heuristic-based extractor for Swift.
package swiftextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// SwiftExtractor implements extractor.Extractor for Swift files.
type SwiftExtractor struct{}

// New creates a new SwiftExtractor.
func New() *SwiftExtractor {
	return &SwiftExtractor{}
}

func (s *SwiftExtractor) Language() string { return "swift" }

func (s *SwiftExtractor) Supports(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".swift"
}

func (s *SwiftExtractor) SupportedKinds() []string {
	return []string{"module", "class", "struct", "enum", "protocol", "extension", "function", "method", "property", "const", "test"}
}

func (s *SwiftExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	// Module name derived from file path: dots as separators, sans extension.
	moduleName := deriveModuleName(req.FilePath)

	// Package name is the file base name without extension.
	pkgName := strings.TrimSuffix(filepath.Base(req.FilePath), filepath.Ext(req.FilePath))

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          pkgName,
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "swift",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	return result, nil
}

// deriveModuleName converts a file path to a dot-separated module name.
// E.g. "Sources/Models/User.swift" -> "Sources.Models.User"
func deriveModuleName(filePath string) string {
	name := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	return strings.ReplaceAll(name, string(filepath.Separator), ".")
}

// Regex patterns for Swift extraction.
var (
	importRe       = regexp.MustCompile(`(?m)^import\s+(\w+)`)
	classDeclRe    = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+|open\s+)?(?:final\s+)?class\s+(\w+)`)
	structDeclRe   = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?struct\s+(\w+)`)
	enumDeclRe     = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?(?:indirect\s+)?enum\s+(\w+)`)
	protocolRe     = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?protocol\s+(\w+)`)
	extensionRe    = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?extension\s+(\w+)`)
	funcDeclRe     = regexp.MustCompile(`(?m)^(?:\s*)(?:public\s+|private\s+|internal\s+|fileprivate\s+|open\s+)?(?:static\s+|class\s+)?(?:override\s+)?func\s+(\w+)`)
	propertyDeclRe = regexp.MustCompile(`(?m)^(?:\s*)(?:public\s+|private\s+|internal\s+|fileprivate\s+)?(?:static\s+|class\s+)?(?:lazy\s+)?(?:var|let)\s+(\w+)`)
	xctestRe       = regexp.MustCompile(`(?m)^\s+(?:override\s+)?func\s+(test\w+)\s*\(`)
	swiftTestRe    = regexp.MustCompile(`(?m)@Test\s*\n\s*(?:public\s+|private\s+)?func\s+(\w+)`)
)

func extractImports(content string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	matches := importRe.FindAllStringSubmatchIndex(content, -1)
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

	// Collect @Test-annotated method names for deduplication.
	swiftTestNames := make(map[string]bool)
	for _, m := range swiftTestRe.FindAllStringSubmatch(content, -1) {
		swiftTestNames[m[1]] = true
	}

	// Collect XCTest method names (test* prefix) for deduplication.
	xctestNames := make(map[int]string) // line number -> method name
	for _, m := range xctestRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		methodLine := strings.Count(content[:m[2]], "\n") + 1
		xctestNames[methodLine] = name
	}

	// Type containers: classes, structs, enums, protocols, extensions.
	type container struct {
		name      string
		kind      string
		startLine int
		endLine   int
		qname     string
	}
	var containers []container

	// Helper to extract a container type.
	extractContainer := func(re *regexp.Regexp, kind string) {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			line := strings.Count(content[:m[0]], "\n") + 1
			endLine := findBlockEnd(lines, line-1)
			lineContent := ""
			if line-1 < len(lines) {
				lineContent = lines[line-1]
			}

			qname := moduleName + "." + name
			stableID := "swift:" + qname + ":" + kind

			symbols = append(symbols, extractor.SymbolRecord{
				Name:          name,
				QualifiedName: qname,
				SymbolKind:    kind,
				Visibility:    visibility(lineContent),
				StartLine:     line,
				EndLine:       endLine,
				StableID:      stableID,
			})

			containers = append(containers, container{
				name:      name,
				kind:      kind,
				startLine: line,
				endLine:   endLine,
				qname:     qname,
			})
		}
	}

	extractContainer(classDeclRe, "class")
	extractContainer(structDeclRe, "struct")
	extractContainer(enumDeclRe, "enum")
	extractContainer(protocolRe, "protocol")
	extractContainer(extensionRe, "extension")

	// Top-level functions (not inside any container).
	for _, m := range funcDeclRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		// Skip indented declarations — those are methods inside containers.
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}

		// Skip if this line falls within any container's range.
		insideContainer := false
		for _, c := range containers {
			if line > c.startLine && line < c.endLine {
				insideContainer = true
				break
			}
		}
		if insideContainer {
			continue
		}

		qname := moduleName + "." + name
		stableID := "swift:" + qname + ":function"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "function",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       findBlockEnd(lines, line-1),
			StableID:      stableID,
		})
	}

	// Top-level properties: let with ALL_CAPS name -> "const", otherwise skip.
	for _, m := range propertyDeclRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		// Only process top-level (non-indented) properties here.
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}

		// Top-level let with ALL_CAPS is a constant.
		if isAllCaps(name) && strings.Contains(lineContent, "let ") {
			qname := moduleName + "." + name
			stableID := "swift:" + qname + ":const"
			symbols = append(symbols, extractor.SymbolRecord{
				Name:          name,
				QualifiedName: qname,
				SymbolKind:    "const",
				Visibility:    visibility(lineContent),
				StartLine:     line,
				EndLine:       line,
				StableID:      stableID,
			})
		}
	}

	// Track which lines have test symbols to avoid duplicates.
	testSymbolLines := make(map[int]bool)

	// Extract members within containers.
	for _, c := range containers {
		for i := c.startLine; i < min(c.endLine, len(lines)); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "}" || trimmed == "{" {
				continue
			}

			// Check for methods (func declarations inside containers).
			if fm := funcDeclRe.FindStringSubmatch(line); fm != nil {
				// Must be indented to be a method.
				if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
					continue
				}
				methodName := fm[1]
				methodLine := i + 1 // 1-indexed

				kind := "method"

				// XCTest detection: method starts with "test".
				if _, ok := xctestNames[methodLine]; ok {
					kind = "test"
				}

				// Swift Testing detection: @Test attribute.
				if swiftTestNames[methodName] {
					// Verify the @Test is actually preceding this declaration.
					if isSwiftTestMethod(lines, i) {
						kind = "test"
					}
				}

				// Also check XCTest naming convention directly.
				if kind == "method" && strings.HasPrefix(methodName, "test") {
					if xctestRe.MatchString(line) {
						kind = "test"
					}
				}

				if testSymbolLines[methodLine] {
					continue
				}
				if kind == "test" {
					testSymbolLines[methodLine] = true
				}

				qname := c.qname + "." + methodName
				stableID := "swift:" + qname + ":" + kind

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           methodName,
					QualifiedName:  qname,
					SymbolKind:     kind,
					Visibility:     visibility(line),
					ParentSymbolID: c.qname,
					StartLine:      methodLine,
					EndLine:        findBlockEnd(lines, i),
					StableID:       stableID,
				})
				continue
			}

			// Check for properties (var/let inside containers).
			if pm := propertyDeclRe.FindStringSubmatch(line); pm != nil {
				// Must be indented.
				if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
					continue
				}
				propName := pm[1]

				qname := c.qname + "." + propName
				stableID := "swift:" + qname + ":property"

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           propName,
					QualifiedName:  qname,
					SymbolKind:     "property",
					Visibility:     visibility(line),
					ParentSymbolID: c.qname,
					StartLine:      i + 1,
					EndLine:        i + 1,
					StableID:       stableID,
				})
			}
		}
	}

	return symbols
}

// visibility determines if a declaration is exported (public/open) or unexported.
func visibility(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "public") || strings.HasPrefix(trimmed, "open") {
		return "exported"
	}
	return "unexported"
}

// isAllCaps checks if a name matches ALL_CAPS pattern (uppercase letters, digits, underscores).
func isAllCaps(name string) bool {
	if name == "" {
		return false
	}
	for _, ch := range name {
		if ch != '_' && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

// isSwiftTestMethod checks if a func declaration at lineIdx is preceded by @Test.
func isSwiftTestMethod(lines []string, lineIdx int) bool {
	for i := lineIdx - 1; i >= 0 && i >= lineIdx-3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "@Test" {
			return true
		}
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "@") {
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
