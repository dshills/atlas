package luaextractor

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

		// Skip busted describe/it blocks — they don't have a simple function name to match.
		if strings.Contains(s.Name, "_") && !strings.HasPrefix(strings.ToLower(s.Name), "test") {
			continue
		}

		matched := ""

		// Try stripping "test" prefix (test_create -> create, testCreate -> create).
		if strings.HasPrefix(s.Name, "test_") {
			candidate := strings.TrimPrefix(s.Name, "test_")
			if orig, ok := nonTestNames[strings.ToLower(candidate)]; ok {
				matched = orig
			}
		}

		// Try stripping "Test" prefix (TestUser -> User).
		if matched == "" && strings.HasPrefix(s.Name, "Test") {
			candidate := strings.TrimPrefix(s.Name, "Test")
			if orig, ok := nonTestNames[strings.ToLower(candidate)]; ok {
				matched = orig
			}
			// Try lowercasing first char (TestFoo -> foo)
			if matched == "" && len(candidate) > 0 {
				lower := strings.ToLower(candidate[:1]) + candidate[1:]
				if orig, ok := nonTestNames[strings.ToLower(lower)]; ok {
					matched = orig
				}
			}
		}

		// Try stripping "test" prefix without underscore (testcreate -> create).
		if matched == "" && strings.HasPrefix(strings.ToLower(s.Name), "test") && !strings.HasPrefix(s.Name, "test_") && !strings.HasPrefix(s.Name, "Test") {
			candidate := s.Name[4:]
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
			Confidence:     "likely",
			Line:           s.StartLine,
			RawTargetText:  matched,
		})
	}

	return refs
}
