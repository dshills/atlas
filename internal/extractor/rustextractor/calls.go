package rustextractor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

var (
	rustDirectCallRe   = regexp.MustCompile(`(?m)(?:^|[^:.\w])([a-z_]\w*)\s*\(`)
	rustPathCallRe     = regexp.MustCompile(`(?m)(\w+(?:::\w+)+)\s*\(`)
	rustSelectorCallRe = regexp.MustCompile(`(?m)(\w+)\.(\w+)\s*\(`)
	rustMacroCallRe    = regexp.MustCompile(`(?m)(\w+)!\s*[\(\[\{]`)
)

var rustKeywords = map[string]bool{
	"fn": true, "let": true, "mut": true, "const": true, "static": true,
	"struct": true, "enum": true, "trait": true, "type": true, "impl": true,
	"mod": true, "use": true, "pub": true, "if": true, "else": true,
	"for": true, "while": true, "loop": true, "match": true, "return": true,
	"where": true, "as": true, "in": true, "ref": true, "move": true,
	"break": true, "continue": true, "unsafe": true, "async": true,
	"await": true, "dyn": true, "extern": true, "crate": true,
	"self": true, "super": true,
}

var rustMacroExclusions = map[string]bool{
	"println": true, "eprintln": true, "dbg": true, "format": true,
	"vec": true, "assert": true, "assert_eq": true, "assert_ne": true,
	"cfg": true, "derive": true, "todo": true, "unimplemented": true,
	"unreachable": true, "panic": true, "write": true, "writeln": true,
	"include": true, "include_str": true, "include_bytes": true,
	"env": true, "concat": true, "stringify": true, "line": true,
	"file": true, "column": true, "module_path": true,
}

func extractCalls(content string, _ []string, codeLines []bool, _ []extractor.SymbolRecord, _ string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord
	seen := map[string]bool{}

	// Direct function calls
	for _, m := range rustDirectCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if rustKeywords[name] {
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

	// Path calls (e.g. std::fs::read_to_string)
	for _, m := range rustPathCallRe.FindAllStringSubmatchIndex(content, -1) {
		path := content[m[2]:m[3]]
		line := strings.Count(content[:m[2]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		key := fmt.Sprintf("%d:%s", line, path)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, extractor.ReferenceRecord{
			ReferenceKind: "calls",
			Confidence:    "heuristic",
			Line:          line,
			RawTargetText: path,
		})
	}

	// Selector calls (e.g. client.send())
	for _, m := range rustSelectorCallRe.FindAllStringSubmatchIndex(content, -1) {
		receiver := content[m[2]:m[3]]
		method := content[m[4]:m[5]]
		if rustKeywords[method] {
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

	// Macro calls (e.g. custom_macro!(args))
	for _, m := range rustMacroCallRe.FindAllStringSubmatchIndex(content, -1) {
		name := content[m[2]:m[3]]
		if rustMacroExclusions[name] {
			continue
		}
		line := strings.Count(content[:m[2]], "\n") + 1
		if line-1 >= len(codeLines) || !codeLines[line-1] {
			continue
		}
		target := name + "!"
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
