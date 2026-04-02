package pyextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	pyDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^.\w])([a-z_]\w*)\s*\(`)
	pySelectorCallRe = regexp.MustCompile(`(?m)(\w+)\.(\w+)\s*\(`)
)

var pyKeywords = map[string]bool{
	"def": true, "class": true, "if": true, "elif": true, "for": true,
	"while": true, "with": true, "except": true, "import": true, "from": true,
	"as": true, "and": true, "or": true, "not": true, "in": true, "is": true,
	"lambda": true, "assert": true, "raise": true, "return": true, "yield": true,
	"del": true, "global": true, "nonlocal": true, "pass": true, "break": true,
	"continue": true, "try": true, "finally": true, "else": true,
}

var pyBuiltins = map[string]bool{
	"print": true, "len": true, "range": true, "str": true, "int": true,
	"float": true, "bool": true, "list": true, "dict": true, "set": true,
	"tuple": true, "type": true, "super": true, "isinstance": true,
	"issubclass": true, "hasattr": true, "getattr": true, "setattr": true,
	"delattr": true, "repr": true, "hash": true, "id": true, "abs": true,
	"min": true, "max": true, "sum": true, "sorted": true, "reversed": true,
	"enumerate": true, "zip": true, "map": true, "filter": true, "open": true,
	"input": true, "iter": true, "next": true, "any": true, "all": true,
	"dir": true, "vars": true, "globals": true, "locals": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls: identifier(
	for _, m := range pyDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if pyKeywords[name] || pyBuiltins[name] {
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
	for _, m := range pySelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if pyKeywords[method] {
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
