package luaextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	luaDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^.\w:])([a-z_]\w*)\s*\(`)
	luaSelectorCallRe = regexp.MustCompile(`(?m)([a-zA-Z_]\w*)\.([a-zA-Z_]\w*)\s*\(`)
	luaColonCallRe    = regexp.MustCompile(`(?m)([a-zA-Z_]\w*):([a-zA-Z_]\w*)\s*\(`)
)

var luaKeywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true,
	"end": true, "false": true, "for": true, "function": true, "goto": true,
	"if": true, "in": true, "local": true, "nil": true, "not": true,
	"or": true, "repeat": true, "return": true, "then": true, "true": true,
	"until": true, "while": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls: identifier(
	for _, m := range luaDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if luaKeywords[name] {
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
	for _, m := range luaSelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if luaKeywords[method] {
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

	// Colon calls: obj:method() — Lua's method call syntax
	for _, m := range luaColonCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if luaKeywords[method] {
			continue
		}
		line := strings.Count(content[:m[0]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		target := receiver + ":" + method
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
