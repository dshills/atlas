package csharpextractor

import (
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// extractTestReferences generates "tests" references by matching test symbols
// against non-test symbols based on name heuristics.
func extractTestReferences(symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord {
	// Build a set of non-test symbol names for matching.
	nonTestNames := make(map[string]bool)
	for _, s := range symbols {
		if s.SymbolKind != "test" {
			nonTestNames[s.Name] = true
		}
	}

	var refs []extractor.ReferenceRecord

	for _, s := range symbols {
		if s.SymbolKind != "test" {
			continue
		}

		matched := ""

		// Try direct name match.
		if nonTestNames[s.Name] {
			matched = s.Name
		}

		// Try stripping common test prefixes.
		if matched == "" {
			for _, prefix := range []string{"test", "Test"} {
				if strings.HasPrefix(s.Name, prefix) {
					candidate := strings.TrimPrefix(s.Name, prefix)
					if nonTestNames[candidate] {
						matched = candidate
						break
					}
					// Try lowercasing first char.
					if len(candidate) > 0 {
						lower := strings.ToLower(candidate[:1]) + candidate[1:]
						if nonTestNames[lower] {
							matched = lower
							break
						}
					}
				}
			}
		}

		if matched == "" {
			continue
		}

		refs = append(refs, extractor.ReferenceRecord{
			FromSymbolName: s.QualifiedName,
			ToSymbolName:   moduleName + "." + matched,
			ReferenceKind:  "tests",
			Confidence:     "likely",
			Line:           s.StartLine,
			RawTargetText:  matched,
		})
	}

	return refs
}
