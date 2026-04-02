// Package csharpextractor implements a regex/heuristic-based extractor for C#.
package csharpextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// CSharpExtractor implements extractor.Extractor for C# files.
type CSharpExtractor struct{}

// New creates a new CSharpExtractor.
func New() *CSharpExtractor {
	return &CSharpExtractor{}
}

func (c *CSharpExtractor) Language() string { return "csharp" }

func (c *CSharpExtractor) Supports(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".cs"
}

func (c *CSharpExtractor) SupportedKinds() []string {
	return []string{"namespace", "class", "interface", "enum", "struct", "method", "property", "field", "const", "record", "test"}
}

func (c *CSharpExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	// Derive module name from namespace declaration, fallback to file path
	moduleName := deriveModuleName(content, req.FilePath)

	// Package name is the last segment of the dotted namespace path
	pkgName := moduleName
	if idx := strings.LastIndex(moduleName, "."); idx >= 0 {
		pkgName = moduleName[idx+1:]
	}

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          pkgName,
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "csharp",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	return result, nil
}

func deriveModuleName(content, filePath string) string {
	m := namespaceDeclRe.FindStringSubmatch(content)
	if m != nil {
		return m[1]
	}
	// Fallback: file path without extension, separators replaced with dots
	name := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	return strings.ReplaceAll(name, string(filepath.Separator), ".")
}

// Regex patterns for C# extraction.
var (
	namespaceDeclRe = regexp.MustCompile(`(?m)^namespace\s+([\w.]+)`)
	usingRe         = regexp.MustCompile(`(?m)^using\s+(?:static\s+)?([^;]+);`)
	classDeclRe     = regexp.MustCompile(`(?m)^[ \t]*(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:abstract\s+)?(?:sealed\s+)?(?:partial\s+)?class\s+(\w+)`)
	interfaceDeclRe = regexp.MustCompile(`(?m)^[ \t]*(?:public\s+|private\s+|protected\s+|internal\s+)?(?:partial\s+)?interface\s+(\w+)`)
	enumDeclRe      = regexp.MustCompile(`(?m)^[ \t]*(?:public\s+|private\s+|protected\s+|internal\s+)?enum\s+(\w+)`)
	structDeclRe    = regexp.MustCompile(`(?m)^[ \t]*(?:public\s+|private\s+|protected\s+|internal\s+)?(?:readonly\s+)?(?:partial\s+)?struct\s+(\w+)`)
	recordDeclRe    = regexp.MustCompile(`(?m)^[ \t]*(?:public\s+|private\s+|protected\s+|internal\s+)?(?:sealed\s+)?record\s+(?:struct\s+|class\s+)?(\w+)`)
	methodDeclRe    = regexp.MustCompile(`(?m)^[ \t]+(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:virtual\s+|override\s+|abstract\s+|async\s+)?(?:\w+(?:<[^>]*>)?(?:\[\]|\?)?)\s+(\w+)\s*[(<]`)
	constFieldRe    = regexp.MustCompile(`(?m)^[ \t]+(?:public\s+|private\s+|protected\s+|internal\s+)?const\s+\w+\s+(\w+)\s*=`)
	propertyDeclRe  = regexp.MustCompile(`(?m)^[ \t]+(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:virtual\s+|override\s+|abstract\s+)?(?:\w+(?:<[^>]*>)?(?:\[\]|\?)?)\s+(\w+)\s*\{`)
	testAttrRe      = regexp.MustCompile(`(?m)\[(Test|Fact|Theory|TestMethod)\]\s*\n\s*(?:public\s+|private\s+)?(?:static\s+)?(?:async\s+)?(?:Task\s+|void\s+)(\w+)`)
)

func extractImports(content string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	matches := usingRe.FindAllStringSubmatchIndex(content, -1)
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

	// Collect test method lines from attribute-annotated tests
	testMethodLines := make(map[int]string) // line number -> method name
	testMatches := testAttrRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range testMatches {
		name := content[m[4]:m[5]]
		methodLine := strings.Count(content[:m[4]], "\n") + 1
		testMethodLines[methodLine] = name
	}

	// Type containers: classes, interfaces, enums, structs, records
	type container struct {
		name      string
		kind      string
		startLine int
		endLine   int
		qname     string
	}
	var containers []container

	// Helper to extract type containers with a given regex and kind
	extractContainers := func(re *regexp.Regexp, kind string) {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			line := strings.Count(content[:m[0]], "\n") + 1
			endLine := findBlockEnd(lines, line-1)
			lineContent := ""
			if line-1 < len(lines) {
				lineContent = lines[line-1]
			}

			qname := moduleName + "." + name
			stableID := "csharp:" + qname + ":" + kind

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

	extractContainers(classDeclRe, "class")
	extractContainers(interfaceDeclRe, "interface")
	extractContainers(enumDeclRe, "enum")
	extractContainers(structDeclRe, "struct")
	extractContainers(recordDeclRe, "record")

	// Track which lines already have test symbols (for deduplication)
	testSymbolLines := make(map[int]bool)

	// Extract methods, properties, and constants within containers
	for _, ct := range containers {
		for i := ct.startLine; i < min(ct.endLine, len(lines)); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "}" || trimmed == "{" {
				continue
			}

			// Check for constants
			if cm := constFieldRe.FindStringSubmatch(line); cm != nil {
				constName := cm[1]
				qname := ct.qname + "." + constName
				stableID := "csharp:" + qname + ":const"

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           constName,
					QualifiedName:  qname,
					SymbolKind:     "const",
					Visibility:     visibility(line),
					ParentSymbolID: ct.qname,
					StartLine:      i + 1,
					EndLine:        i + 1,
					StableID:       stableID,
				})
				continue
			}

			// Check for properties (must check before methods since they overlap in pattern)
			if pm := propertyDeclRe.FindStringSubmatch(line); pm != nil {
				propName := pm[1]
				propLine := i + 1

				// Skip if this looks like a method line too (properties have '{', methods have '(')
				// The propertyDeclRe requires '{' so this should be correct
				qname := ct.qname + "." + propName
				stableID := "csharp:" + qname + ":property"

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           propName,
					QualifiedName:  qname,
					SymbolKind:     "property",
					Visibility:     visibility(line),
					ParentSymbolID: ct.qname,
					StartLine:      propLine,
					EndLine:        propLine,
					StableID:       stableID,
				})
				continue
			}

			// Check for methods
			if mm := methodDeclRe.FindStringSubmatch(line); mm != nil {
				methodName := mm[1]
				methodLine := i + 1

				// Determine if this is a test method
				kind := "method"

				// Check test attribute annotations
				if _, ok := testMethodLines[methodLine]; ok {
					kind = "test"
				}

				// Check annotation on previous lines (look back up to 3 lines)
				if kind == "method" {
					for j := i - 1; j >= 0 && j >= i-3; j-- {
						prevTrimmed := strings.TrimSpace(lines[j])
						if prevTrimmed == "" {
							continue
						}
						if prevTrimmed == "[Test]" || prevTrimmed == "[Fact]" || prevTrimmed == "[Theory]" || prevTrimmed == "[TestMethod]" {
							kind = "test"
							break
						}
						if !strings.HasPrefix(prevTrimmed, "[") {
							break
						}
					}
				}

				if testSymbolLines[methodLine] {
					continue
				}
				if kind == "test" {
					testSymbolLines[methodLine] = true
				}

				qname := ct.qname + "." + methodName
				stableID := "csharp:" + qname + ":" + kind

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           methodName,
					QualifiedName:  qname,
					SymbolKind:     kind,
					Visibility:     visibility(line),
					ParentSymbolID: ct.qname,
					StartLine:      methodLine,
					EndLine:        findBlockEnd(lines, i),
					StableID:       stableID,
				})
			}
		}
	}

	return symbols
}

// visibility determines if a declaration is exported (public/internal) or unexported.
func visibility(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(trimmed, "public ") || strings.HasPrefix(trimmed, "public ") {
		return "exported"
	}
	if strings.Contains(trimmed, "internal ") || strings.HasPrefix(trimmed, "internal ") {
		return "exported"
	}
	return "unexported"
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
