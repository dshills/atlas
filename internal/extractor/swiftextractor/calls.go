package swiftextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	swiftDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^.\w])([a-z_]\w*)\s*\(`)
	swiftSelectorCallRe = regexp.MustCompile(`(?m)([a-zA-Z_]\w*)\.([a-zA-Z_]\w*)\s*\(`)
)

var swiftKeywords = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true, "continue": true,
	"default": true, "defer": true, "do": true, "else": true, "enum": true,
	"extension": true, "fallthrough": true, "for": true, "func": true, "guard": true,
	"if": true, "import": true, "in": true, "init": true, "inout": true,
	"is": true, "let": true, "operator": true, "protocol": true, "repeat": true,
	"return": true, "self": true, "static": true, "struct": true, "subscript": true,
	"super": true, "switch": true, "throw": true, "throws": true, "try": true,
	"typealias": true, "var": true, "where": true, "while": true, "as": true,
	"await": true, "async": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls
	for _, m := range swiftDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if swiftKeywords[name] {
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

	// Selector calls: receiver.method(...)
	for _, m := range swiftSelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if swiftKeywords[method] {
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
