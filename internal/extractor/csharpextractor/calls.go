package csharpextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	csharpDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^.\w])([a-z_]\w*)\s*\(`)
	csharpSelectorCallRe = regexp.MustCompile(`(?m)([a-zA-Z_]\w*)\.([a-zA-Z_]\w*)\s*\(`)
)

var csharpKeywords = map[string]bool{
	"abstract": true, "as": true, "base": true, "break": true, "case": true,
	"catch": true, "checked": true, "class": true, "continue": true, "default": true,
	"delegate": true, "do": true, "else": true, "enum": true, "event": true,
	"explicit": true, "extern": true, "finally": true, "fixed": true, "for": true,
	"foreach": true, "goto": true, "if": true, "implicit": true, "in": true,
	"interface": true, "internal": true, "is": true, "lock": true, "namespace": true,
	"new": true, "operator": true, "out": true, "override": true, "params": true,
	"private": true, "protected": true, "public": true, "readonly": true, "ref": true,
	"return": true, "sealed": true, "sizeof": true, "stackalloc": true, "static": true,
	"struct": true, "switch": true, "this": true, "throw": true, "try": true,
	"typeof": true, "unchecked": true, "unsafe": true, "using": true, "virtual": true,
	"void": true, "volatile": true, "while": true, "yield": true, "var": true,
	"await": true, "async": true, "record": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls: identifier(
	for _, m := range csharpDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if csharpKeywords[name] {
			continue
		}
		line := strings.Count(content[:m[2]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		key := fmt.Sprintf("%d:%s", line, name)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "calls",
			Confidence:    "heuristic",
			Line:          line,
			RawTargetText: name,
		})
	}

	// Selector calls: receiver.method(
	for _, m := range csharpSelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if csharpKeywords[method] {
			continue
		}
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		target := receiver + "." + method
		key := fmt.Sprintf("%d:%s", line, target)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "calls",
			Confidence:    "heuristic",
			Line:          line,
			RawTargetText: target,
		})
	}

	return refs
}
