package pyextractor

import (
	"strings"

	"github.com/dshills/atlas/internal/extractor"
)

// extractTestReferences generates "tests" references by matching test symbols
// against non-test symbols based on name heuristics.
func extractTestReferences(symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord {
	// Build a set of non-test symbol names for case-insensitive matching.
	nonTestNames := make(map[string]string) // lowercase -> original name
	for _, s := range symbols {
		if s.SymbolKind != "test" {
			nonTestNames[strings.ToLower(s.Name)] = s.Name
		}
	}

	var refs []extractor.ReferenceRecord

	for _, s := range symbols {
		if s.SymbolKind != "test" {
			continue
		}

		matched := ""

		// Try stripping "test_" prefix (Python convention: test_foo -> foo).
		if strings.HasPrefix(s.Name, "test_") {
			candidate := strings.TrimPrefix(s.Name, "test_")
			if orig, ok := nonTestNames[strings.ToLower(candidate)]; ok {
				matched = orig
			}
		}

		// Try stripping "Test" prefix (class-based tests: TestUser -> User).
		if matched == "" && strings.HasPrefix(s.Name, "Test") {
			candidate := strings.TrimPrefix(s.Name, "Test")
			if orig, ok := nonTestNames[strings.ToLower(candidate)]; ok {
				matched = orig
			}
		}

		// Try direct name match.
		if matched == "" {
			if orig, ok := nonTestNames[strings.ToLower(s.Name)]; ok {
				matched = orig
			}
		}

		if matched == "" {
			continue
		}

		refs = append(refs, extractor.ReferenceRecord{
			FromSymbolName: s.QualifiedName,
			ToSymbolName:   moduleName + "." + matched,
			ReferenceKind:  "tests",
			Confidence:     "heuristic",
			Line:           s.StartLine,
			RawTargetText:  matched,
		})
	}

	return refs
}
