package javaextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	javaDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^.\w])([a-z_]\w*)\s*\(`)
	javaSelectorCallRe = regexp.MustCompile(`(?m)([a-zA-Z_]\w*)\.([a-zA-Z_]\w*)\s*\(`)
)

var javaKeywords = map[string]bool{
	"abstract": true, "assert": true, "break": true, "case": true, "catch": true,
	"class": true, "continue": true, "default": true, "do": true, "else": true,
	"enum": true, "extends": true, "final": true, "finally": true, "for": true,
	"if": true, "implements": true, "import": true, "instanceof": true, "interface": true,
	"native": true, "new": true, "package": true, "private": true, "protected": true,
	"public": true, "return": true, "static": true, "strictfp": true, "super": true,
	"switch": true, "synchronized": true, "this": true, "throw": true, "throws": true,
	"transient": true, "try": true, "volatile": true, "while": true, "yield": true,
	"var": true, "void": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls: identifier(
	for _, m := range javaDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if javaKeywords[name] {
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
	for _, m := range javaSelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if javaKeywords[method] {
			continue
		}
		line := strings.Count(content[:m[2]], "\n") + 1
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
