// Package javaextractor implements a regex/heuristic-based extractor for Java.
package javaextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// JavaExtractor implements extractor.Extractor for Java files.
type JavaExtractor struct{}

// New creates a new JavaExtractor.
func New() *JavaExtractor {
	return &JavaExtractor{}
}

func (j *JavaExtractor) Language() string { return "java" }

func (j *JavaExtractor) Supports(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".java"
}

func (j *JavaExtractor) SupportedKinds() []string {
	return []string{"package", "class", "interface", "enum", "method", "field", "const", "annotation", "test"}
}

func (j *JavaExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	// Derive module name from package declaration, fallback to file path
	moduleName := deriveModuleName(content, req.FilePath)

	// Package name is the last segment of the dotted package path
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
			Language:      "java",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	return result, nil
}

func deriveModuleName(content, filePath string) string {
	m := packageRe.FindStringSubmatch(content)
	if m != nil {
		return m[1]
	}
	// Fallback: file path without extension, separators replaced with dots
	name := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	return strings.ReplaceAll(name, string(filepath.Separator), ".")
}

// Regex patterns for Java extraction.
var (
	packageRe    = regexp.MustCompile(`(?m)^package\s+([\w.]+);`)
	importRe     = regexp.MustCompile(`(?m)^import\s+(?:static\s+)?([^;]+);`)
	classDeclRe  = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+)?(?:abstract\s+)?(?:final\s+)?(?:static\s+)?class\s+(\w+)`)
	interfaceRe  = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+)?interface\s+(\w+)`)
	enumDeclRe   = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+)?enum\s+(\w+)`)
	methodDeclRe = regexp.MustCompile(`(?m)^\s+(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:abstract\s+)?(?:final\s+)?(?:synchronized\s+)?(?:<[^>]+>\s+)?(?:\w+(?:<[^>]*>)?(?:\[\])*)\s+(\w+)\s*\(`)
	constFieldRe = regexp.MustCompile(`(?m)^\s+(?:public\s+|private\s+|protected\s+)?static\s+final\s+\w+\s+([A-Z_][A-Z0-9_]*)\s*=`)
	annotationRe = regexp.MustCompile(`(?m)^(?:public\s+)?@interface\s+(\w+)`)
	junitTestRe  = regexp.MustCompile(`(?m)@(?:Test|ParameterizedTest)\s*(?:\([^)]*\))?\s*\n\s*(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:void\s+)(\w+)`)
	testCaseRe   = regexp.MustCompile(`(?m)class\s+\w+\s+extends\s+TestCase`)
	testMethodRe = regexp.MustCompile(`(?m)^\s+(?:public\s+)?void\s+(test\w+)\s*\(`)
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

	// Determine if this file extends TestCase (for legacy JUnit 3 style tests)
	isTestCaseFile := testCaseRe.MatchString(content)

	// Collect JUnit-annotated test method lines for deduplication
	junitTestLines := make(map[int]string) // line number -> method name
	junitMatches := junitTestRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range junitMatches {
		name := content[m[2]:m[3]]
		// The captured group is the method name on the line after the annotation.
		// Use the position of the captured name to determine the line number.
		methodLine := strings.Count(content[:m[2]], "\n") + 1
		junitTestLines[methodLine] = name
	}

	// Type containers: classes, interfaces, enums
	type container struct {
		name      string
		kind      string
		startLine int
		endLine   int
		qname     string
	}
	var containers []container

	// Classes
	for _, m := range classDeclRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		endLine := findBlockEnd(lines, line-1)
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "." + name
		stableID := "java:" + qname + ":class"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "class",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      stableID,
		})

		containers = append(containers, container{
			name:      name,
			kind:      "class",
			startLine: line,
			endLine:   endLine,
			qname:     qname,
		})
	}

	// Interfaces
	for _, m := range interfaceRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		endLine := findBlockEnd(lines, line-1)
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "." + name
		stableID := "java:" + qname + ":interface"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "interface",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      stableID,
		})

		containers = append(containers, container{
			name:      name,
			kind:      "interface",
			startLine: line,
			endLine:   endLine,
			qname:     qname,
		})
	}

	// Enums
	for _, m := range enumDeclRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		endLine := findBlockEnd(lines, line-1)
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "." + name
		stableID := "java:" + qname + ":enum"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "enum",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      stableID,
		})

		containers = append(containers, container{
			name:      name,
			kind:      "enum",
			startLine: line,
			endLine:   endLine,
			qname:     qname,
		})
	}

	// Annotation types (@interface)
	for _, m := range annotationRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		endLine := findBlockEnd(lines, line-1)
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}

		qname := moduleName + "." + name
		stableID := "java:" + qname + ":annotation"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "annotation",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      stableID,
		})
	}

	// Track which lines already have test symbols (for deduplication)
	testSymbolLines := make(map[int]bool)

	// Extract methods and constants within containers
	for _, c := range containers {
		for i := c.startLine; i < min(c.endLine, len(lines)); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "}" || trimmed == "{" {
				continue
			}

			// Check for constants (static final)
			if cm := constFieldRe.FindStringSubmatch(line); cm != nil {
				constName := cm[1]
				qname := c.qname + "." + constName
				stableID := "java:" + qname + ":const"

				symbols = append(symbols, extractor.SymbolRecord{
					Name:           constName,
					QualifiedName:  qname,
					SymbolKind:     "const",
					Visibility:     visibility(line),
					ParentSymbolID: c.qname,
					StartLine:      i + 1,
					EndLine:        i + 1,
					StableID:       stableID,
				})
				continue
			}

			// Check for methods
			if mm := methodDeclRe.FindStringSubmatch(line); mm != nil {
				methodName := mm[1]
				methodLine := i + 1 // 1-indexed

				// Determine if this is a test method
				kind := "method"

				// Strategy 1: JUnit annotation (@Test, @ParameterizedTest)
				if _, ok := junitTestLines[methodLine]; ok {
					kind = "test"
				}

				// Strategy 2: TestCase extends with test* naming
				if isTestCaseFile && strings.HasPrefix(methodName, "test") {
					if testMethodRe.MatchString(line) {
						kind = "test"
					}
				}

				// Check annotation on previous lines (look back up to 3 lines)
				if kind == "method" {
					for j := i - 1; j >= 0 && j >= i-3; j-- {
						prevTrimmed := strings.TrimSpace(lines[j])
						if prevTrimmed == "" {
							continue
						}
						if strings.HasPrefix(prevTrimmed, "@Test") || strings.HasPrefix(prevTrimmed, "@ParameterizedTest") {
							kind = "test"
							break
						}
						if !strings.HasPrefix(prevTrimmed, "@") {
							break
						}
					}
				}

				if testSymbolLines[methodLine] {
					continue // deduplicate
				}
				if kind == "test" {
					testSymbolLines[methodLine] = true
				}

				qname := c.qname + "." + methodName
				stableID := "java:" + qname + ":" + kind

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
			}
		}
	}

	return symbols
}

// visibility determines if a declaration is exported (public) or unexported.
func visibility(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "public") {
		return "exported"
	}
	// Also check for public after annotations like @Override
	if strings.Contains(trimmed, "public ") {
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
