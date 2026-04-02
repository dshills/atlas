package tsextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	tsDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^.\w])([a-z]\w*)\s*\(`)
	tsSelectorCallRe = regexp.MustCompile(`(?m)(\w+)\.(\w+)\s*\(`)
)

var tsKeywords = map[string]bool{
	"function": true, "class": true, "if": true, "for": true, "while": true,
	"switch": true, "return": true, "import": true, "export": true, "new": true,
	"catch": true, "typeof": true, "instanceof": true, "case": true, "throw": true,
	"await": true, "yield": true, "from": true, "of": true, "else": true,
	"do": true, "try": true, "finally": true, "const": true, "let": true,
	"var": true, "delete": true, "void": true, "in": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls: identifier(
	for _, m := range tsDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if tsKeywords[name] {
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
	for _, m := range tsSelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if tsKeywords[method] {
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
