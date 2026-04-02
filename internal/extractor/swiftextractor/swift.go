// Package swiftextractor implements a regex/heuristic-based extractor for Swift.
package swiftextractor

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
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
	return []string{"class", "struct", "enum", "protocol", "extension", "function", "method", "var", "const", "typealias", "test"}
}

func (s *SwiftExtractor) Extract(_ context.Context, req extractor.ExtractRequest) (*extractor.ExtractResult, error) {
	content := string(req.Content)
	lines := strings.Split(content, "\n")

	moduleName := deriveModuleName(req.FilePath)

	result := &extractor.ExtractResult{
		File: &extractor.FileRecord{ParseStatus: "ok"},
		Package: &extractor.PackageRecord{
			Name:          filepath.Base(strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath))),
			ImportPath:    moduleName,
			DirectoryPath: filepath.Dir(req.FilePath),
			Language:      "swift",
		},
	}

	result.References = append(result.References, extractImports(content)...)
	result.Symbols = append(result.Symbols, extractSymbols(content, lines, moduleName)...)

	// Comment filter for relationship extraction
	codeLines := commentfilter.LineFilter(content, "swift")

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
	name := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.ReplaceAll(name, string(filepath.Separator), ".")
}

// Regex patterns for Swift extraction.
var (
	swiftImportRe    = regexp.MustCompile(`(?m)^import\s+(?:class\s+|struct\s+|enum\s+|protocol\s+|typealias\s+|func\s+)?(\S+)`)
	swiftClassRe     = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?(?:final\s+)?class\s+(\w+)`)
	swiftStructRe    = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?struct\s+(\w+)`)
	swiftEnumRe      = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?enum\s+(\w+)`)
	swiftProtocolRe  = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?protocol\s+(\w+)`)
	swiftExtRe       = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?extension\s+(\w+)`)
	swiftFuncRe      = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?(?:static\s+|class\s+)?(?:override\s+)?func\s+(\w+)`)
	swiftMethodRe    = regexp.MustCompile(`(?m)^\s+(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?(?:static\s+|class\s+)?(?:override\s+)?func\s+(\w+)`)
	swiftTypealiasRe = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?typealias\s+(\w+)`)
)

func extractImports(content string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	for _, m := range swiftImportRe.FindAllStringSubmatchIndex(content, -1) {
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

	// Type containers for method extraction
	type container struct {
		name      string
		kind      string
		startLine int
		endLine   int
		qname     string
	}
	var containers []container

	// Classes
	for _, m := range swiftClassRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		// Skip indented declarations (nested types)
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		endLine := findBlockEnd(lines, line-1)
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "class",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      "swift:" + qname + ":class",
		})
		containers = append(containers, container{name: name, kind: "class", startLine: line, endLine: endLine, qname: qname})
	}

	// Structs
	for _, m := range swiftStructRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		endLine := findBlockEnd(lines, line-1)
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "struct",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      "swift:" + qname + ":struct",
		})
		containers = append(containers, container{name: name, kind: "struct", startLine: line, endLine: endLine, qname: qname})
	}

	// Enums
	for _, m := range swiftEnumRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		endLine := findBlockEnd(lines, line-1)
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "enum",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      "swift:" + qname + ":enum",
		})
		containers = append(containers, container{name: name, kind: "enum", startLine: line, endLine: endLine, qname: qname})
	}

	// Protocols
	for _, m := range swiftProtocolRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		endLine := findBlockEnd(lines, line-1)
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "protocol",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      "swift:" + qname + ":protocol",
		})
		containers = append(containers, container{name: name, kind: "protocol", startLine: line, endLine: endLine, qname: qname})
	}

	// Extensions
	for _, m := range swiftExtRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		endLine := findBlockEnd(lines, line-1)
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "extension",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      "swift:" + qname + ":extension",
		})
		containers = append(containers, container{name: name, kind: "extension", startLine: line, endLine: endLine, qname: qname})
	}

	// Top-level functions
	for _, m := range swiftFuncRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		// Skip indented functions (methods inside containers)
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		endLine := findBlockEnd(lines, line-1)
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "function",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       endLine,
			StableID:      "swift:" + qname + ":function",
		})
	}

	// Type aliases (top-level)
	for _, m := range swiftTypealiasRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		line := strings.Count(content[:m[0]], "\n") + 1
		lineContent := ""
		if line-1 < len(lines) {
			lineContent = lines[line-1]
		}
		if len(lineContent) > 0 && (lineContent[0] == ' ' || lineContent[0] == '\t') {
			continue
		}
		qname := moduleName + "." + name
		symbols = append(symbols, extractor.SymbolRecord{
			Name:          name,
			QualifiedName: qname,
			SymbolKind:    "typealias",
			Visibility:    visibility(lineContent),
			StartLine:     line,
			EndLine:       line,
			StableID:      "swift:" + qname + ":typealias",
		})
	}

	// Methods inside containers
	for _, c := range containers {
		for i := c.startLine; i < min(c.endLine, len(lines)); i++ {
			mm := swiftMethodRe.FindStringSubmatch(lines[i])
			if mm == nil {
				continue
			}
			methodName := mm[1]
			methodLine := i + 1

			kind := "method"
			// Test methods start with "test" and are in XCTestCase subclasses
			if strings.HasPrefix(methodName, "test") {
				kind = "test"
			}

			mVis := "exported"
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "private") || strings.HasPrefix(trimmed, "fileprivate") {
				mVis = "unexported"
			}

			qname := c.qname + "." + methodName
			stableID := "swift:" + qname + ":" + kind

			symbols = append(symbols, extractor.SymbolRecord{
				Name:           methodName,
				QualifiedName:  qname,
				SymbolKind:     kind,
				Visibility:     mVis,
				ParentSymbolID: c.qname,
				StartLine:      methodLine,
				EndLine:        findBlockEnd(lines, i),
				StableID:       stableID,
			})
		}
	}

	return symbols
}

func visibility(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "private") || strings.HasPrefix(trimmed, "fileprivate") {
		return "unexported"
	}
	return "exported"
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
