// Package csharpextractor implements a regex/heuristic-based extractor for C#.
package csharpextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
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
	return []string{"namespace", "class", "interface", "enum", "struct", "method", "field", "const", "test"}
}

func (c *CSharpExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	moduleName := deriveModuleName(content, req.FilePath)

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

	// Comment filter for relationship extraction
	codeLines := commentfilter.LineFilter(content, "csharp")

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

func deriveModuleName(content, filePath string) string {
	m := namespaceRe.FindStringSubmatch(content)
	if m != nil {
		return m[1]
	}
	name := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	return strings.ReplaceAll(name, string(filepath.Separator), ".")
}

// Regex patterns for C# extraction.
var (
	namespaceRe  = regexp.MustCompile(`(?m)^namespace\s+([\w.]+)`)
	usingRe      = regexp.MustCompile(`(?m)^using\s+(?:static\s+)?([^;]+);`)
	classDeclRe  = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)?(?:abstract\s+)?(?:sealed\s+)?(?:static\s+)?(?:partial\s+)?class\s+(\w+)`)
	interfaceRe  = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)?(?:partial\s+)?interface\s+(\w+)`)
	enumDeclRe   = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)?enum\s+(\w+)`)
	structDeclRe = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)?(?:readonly\s+)?(?:partial\s+)?struct\s+(\w+)`)
	methodDeclRe = regexp.MustCompile(`(?m)^\s+(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:virtual\s+)?(?:override\s+)?(?:abstract\s+)?(?:async\s+)?(?:new\s+)?(?:\w+(?:<[^>]*>)?(?:\[\])*(?:\?)?)\s+(\w+)\s*\(`)
	constFieldRe = regexp.MustCompile(`(?m)^\s+(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:readonly\s+)?const\s+\w+\s+([A-Z_][A-Z0-9_]*)\s*=`)
	testAttrRe   = regexp.MustCompile(`(?m)\[(?:Test|Fact|Theory|TestMethod)\s*(?:\([^)]*\))?\]\s*\n\s*(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:async\s+)?(?:void\s+|Task\s+)(\w+)`)
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

	// Collect test method lines for deduplication
	testMethodLines := make(map[int]string)
	testMatches := testAttrRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range testMatches {
		name := content[m[2]:m[3]]
		methodLine := strings.Count(content[:m[2]], "\n") + 1
		testMethodLines[methodLine] = name
	}

	type container struct {
		name      string
		kind      string
		startLine int
		endLine   int
		qname     string
	}
	var containers []container

	type declPattern struct {
		re   *regexp.Regexp
		kind string
	}
	patterns := []declPattern{
		{classDeclRe, "class"},
		{interfaceRe, "interface"},
		{enumDeclRe, "enum"},
		{structDeclRe, "struct"},
	}

	for _, pat := range patterns {
		for _, m := range pat.re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			line := strings.Count(content[:m[0]], "\n") + 1
			endLine := findBlockEnd(lines, line-1)
			lineContent := ""
			if line-1 < len(lines) {
				lineContent = lines[line-1]
			}

			qname := moduleName + "." + name
			stableID := "csharp:" + qname + ":" + pat.kind

			symbols = append(symbols, extractor.SymbolRecord{
				Name:          name,
				QualifiedName: qname,
				SymbolKind:    pat.kind,
				Visibility:    visibility(lineContent),
				StartLine:     line,
				EndLine:       endLine,
				StableID:      stableID,
			})

			containers = append(containers, container{
				name:      name,
				kind:      pat.kind,
				startLine: line,
				endLine:   endLine,
				qname:     qname,
			})
		}
	}

	testSymbolLines := make(map[int]bool)

	for _, ct := range containers {
		for i := ct.startLine; i < min(ct.endLine, len(lines)); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "}" || trimmed == "{" {
				continue
			}

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

			if mm := methodDeclRe.FindStringSubmatch(line); mm != nil {
				methodName := mm[1]
				methodLine := i + 1

				kind := "method"

				// Check test attribute
				if _, ok := testMethodLines[methodLine]; ok {
					kind = "test"
				}

				// Look back up to 3 lines for test attributes
				if kind == "method" {
					for j := i - 1; j >= 0 && j >= i-3; j-- {
						prevTrimmed := strings.TrimSpace(lines[j])
						if prevTrimmed == "" {
							continue
						}
						if strings.HasPrefix(prevTrimmed, "[Test") ||
							strings.HasPrefix(prevTrimmed, "[Fact") ||
							strings.HasPrefix(prevTrimmed, "[Theory") ||
							strings.HasPrefix(prevTrimmed, "[TestMethod") {
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

func visibility(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "public") || strings.Contains(trimmed, "public ") {
		return "exported"
	}
	return "unexported"
}

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
