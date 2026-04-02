package goextractor

import (
	"fmt"
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// extractTestReferences generates "tests" references for Test* and Benchmark* functions.
func extractTestReferences(symbols []extractor.SymbolRecord, pkgName string) []extractor.ReferenceRecord {
	var refs []extractor.ReferenceRecord

	for _, sym := range symbols {
		var targetName string
		switch {
		case sym.SymbolKind == "test" && strings.HasPrefix(sym.Name, "Test"):
			targetName = strings.TrimPrefix(sym.Name, "Test")
		case sym.SymbolKind == "benchmark" && strings.HasPrefix(sym.Name, "Benchmark"):
			targetName = strings.TrimPrefix(sym.Name, "Benchmark")
		default:
			continue
		}

		if targetName == "" {
			continue
		}

		// Strip any trailing underscore-separated suffixes (e.g. TestFoo_Error → Foo)
		if idx := strings.Index(targetName, "_"); idx > 0 {
			targetName = targetName[:idx]
		}

		refs = append(refs, extractor.ReferenceRecord{
			FromSymbolName: sym.QualifiedName,
			ToSymbolName:   fmt.Sprintf("%s.%s", pkgName, targetName),
			ReferenceKind:  "tests",
			Confidence:     "heuristic",
			RawTargetText:  targetName,
			Line:           sym.StartLine,
		})
	}

	return refs
}
