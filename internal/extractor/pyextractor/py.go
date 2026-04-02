// Package pyextractor implements a regex/heuristic-based extractor for Python.
package pyextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// PyExtractor implements extractor.Extractor for Python files.
type PyExtractor struct{}

// New creates a new PyExtractor.
func New() *PyExtractor {
	return &PyExtractor{}
}

func (p *PyExtractor) Language() string { return "python" }

func (p *PyExtractor) Supports(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".py" || ext == ".pyi"
}

func (p *PyExtractor) SupportedKinds() []string {
	return []string{"module", "function", "method", "class", "var", "const", "test", "decorator"}
}

func (p *PyExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	// Module name from file path
	moduleName := strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))
	moduleName = strings.ReplaceAll(moduleName, string(filepath.Separator), ".")

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          filepath.Base(strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))),
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "python",
		},
	}

	isTestFile := isTestFilePath(req.FilePath)

	result.References = append(result.References, extractImports(content, lines)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName, isTestFile)...)

	return result, nil
}

func isTestFilePath(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(strings.TrimSuffix(base, filepath.Ext(base)), "_test")
}

// Regex patterns for Python extraction.
var (
	importRe      = regexp.MustCompile(`(?m)^import\s+(\S+)`)
	fromImportRe  = regexp.MustCompile(`(?m)^from\s+(\S+)\s+import\s+`)
	funcDeclRe    = regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(`)
	asyncFuncRe   = regexp.MustCompile(`(?m)^async\s+def\s+(\w+)\s*\(`)
	classDeclRe   = regexp.MustCompile(`(?m)^class\s+(\w+)`)
	methodRe      = regexp.MustCompile(`(?m)^    def\s+(\w+)\s*\(`)
	asyncMethodRe = regexp.MustCompile(`(?m)^    async\s+def\s+(\w+)\s*\(`)
	constDeclRe   = regexp.MustCompile(`(?m)^([A-Z][A-Z_0-9]+)\s*=\s*`)
	decoratorRe   = regexp.MustCompile(`(?m)^(@\w[\w.]*)\s*(?:\(|$)`)
)

func extractImports(content string, _ []string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord

	for _, re := range []*regexp.Regexp{importRe, fromImportRe} {
		matches := re.FindAllStringSubmatchIndex(content, -1)
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
	}
	return refs
}

func extractSymbols(content string, lines []string, moduleName string, isTestFile bool) []extractor.SymbolRecord {
	var symbols []extractor.SymbolRecord

	// Top-level functions
	for _, re := range []*regexp.Regexp{funcDeclRe, asyncFuncRe} {
		matches := re.FindAllStringSubmatchIndex(content, -1)
		for _, m := range matches {
			name := content[m[2]:m[3]]
			line := strings.Count(content[:m[0]], "\n") + 1

			kind := "function"
			if isTestFile && strings.HasPrefix(name, "test_") {
				kind = "test"
			}

			vis := "exported"
			if strings.HasPrefix(name, "_") {
				vis = "unexported"
			}

			qname := moduleName + "." + name
			stableID := "python:" + qname + ":" + kind

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

	// Classes
	classMatches := classDeclRe.FindAllStringSubmatchIndex(content, -1)
	for _, cm := range classMatches {
		className := content[cm[2]:cm[3]]
		classLine := strings.Count(content[:cm[0]], "\n") + 1
		classEnd := findBlockEnd(lines, classLine-1)

		vis := "exported"
		if strings.HasPrefix(className, "_") {
			vis = "unexported"
		}

		qname := moduleName + "." + className
		stableID := "python:" + qname + ":class"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          className,
			QualifiedName: qname,
			SymbolKind:    "class",
			Visibility:    vis,
			StartLine:     classLine,
			EndLine:       classEnd,
			StableID:      stableID,
		})

		// Extract methods within the class body
		if classLine-1 < len(lines) && classEnd <= len(lines) {
			classBody := strings.Join(lines[classLine:min(classEnd, len(lines))], "\n")
			for _, mre := range []*regexp.Regexp{methodRe, asyncMethodRe} {
				methodMatches := mre.FindAllStringSubmatch(classBody, -1)
				for _, mm := range methodMatches {
					methodName := mm[1]

					mQname := moduleName + "." + className + "." + methodName
					mStableID := "python:" + mQname + ":method"

					mVis := "exported"
					if strings.HasPrefix(methodName, "_") && !strings.HasPrefix(methodName, "__") {
						mVis = "unexported"
					}

					symbols = append(symbols, extractor.SymbolRecord{
						Name:           methodName,
						QualifiedName:  mQname,
						SymbolKind:     "method",
						Visibility:     mVis,
						ParentSymbolID: qname,
						StableID:       mStableID,
					})
				}
			}
		}
	}

	// Module-level constants (ALL_CAPS)
	constMatches := constDeclRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range constMatches {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + "." + name
		stableID := "python:" + qname + ":const"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "const",
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	// Decorators (for detecting routes, etc.)
	decMatches := decoratorRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range decMatches {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1

		qname := moduleName + "." + name
		stableID := "python:" + qname + ":decorator"

		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "decorator",
			Visibility:    "exported",
			StartLine:     line,
			EndLine:       line,
			StableID:      stableID,
		})
	}

	return symbols
}

// findBlockEnd estimates the end of a Python block using indentation.
func findBlockEnd(lines []string, startIdx int) int {
	if startIdx >= len(lines) {
		return startIdx + 1
	}

	// Find the indentation of the block header
	headerIndent := indentLevel(lines[startIdx])

	// Walk forward looking for lines at the same or lesser indent level
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		// Skip blank lines and comment-only lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indentLevel(line) <= headerIndent {
			return i // 1-indexed not needed here, caller adds 1
		}
	}
	return len(lines)
}

func indentLevel(line string) int {
	count := 0
	for _, ch := range line {
		switch ch {
		case ' ':
			count++
		case '\t':
			count += 4
		default:
			return count
		}
	}
	return count
}
