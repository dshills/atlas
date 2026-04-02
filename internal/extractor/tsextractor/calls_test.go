package tsextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractCalls(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantCalls  []string // expected RawTargetText values
		wantAbsent []string // should NOT appear
	}{
		{
			name:      "direct function call",
			content:   "processData(x)",
			wantCalls: []string{"processData"},
		},
		{
			name:      "selector call",
			content:   "service.getData()",
			wantCalls: []string{"service.getData"},
		},
		{
			name:       "if keyword not a call",
			content:    "if (condition) {",
			wantAbsent: []string{"if"},
		},
		{
			name:       "function keyword not a call",
			content:    "function myFunc() {",
			wantAbsent: []string{"function"},
		},
		{
			name:       "comment excluded",
			content:    "// processData(x)",
			wantAbsent: []string{"processData"},
		},
		{
			name:      "no duplicates for same call on same line",
			content:   "processData(a); processData(b)",
			wantCalls: []string{"processData"},
		},
		{
			name:      "multiple different calls",
			content:   "foo(1)\nbar(2)\nobj.method()",
			wantCalls: []string{"foo", "bar", "obj.method"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang := "typescript"
			codeLines := commentfilter.LineFilter(tt.content, lang)

			refs := extractCalls(tt.content, nil, codeLines, []extractor.SymbolRecord{}, "test/module")

			// Build set of returned RawTargetText
			got := map[string]bool{}
			for _, r := range refs {
				if r.ReferenceKind != "calls" {
					t.Errorf("unexpected ReferenceKind %q", r.ReferenceKind)
				}
				if r.Confidence != "heuristic" {
					t.Errorf("unexpected Confidence %q", r.Confidence)
				}
				got[r.RawTargetText] = true
			}

			for _, want := range tt.wantCalls {
				if !got[want] {
					t.Errorf("expected call %q not found; got %v", want, refs)
				}
			}
			for _, absent := range tt.wantAbsent {
				if got[absent] {
					t.Errorf("call %q should not be present but was found", absent)
				}
			}

			// Check no duplicates
			type lineTarget struct {
				line   int
				target string
			}
			seen := map[lineTarget]int{}
			for _, r := range refs {
				k := lineTarget{r.Line, r.RawTargetText}
				seen[k]++
				if seen[k] > 1 {
					t.Errorf("duplicate call reference: line=%d target=%q", r.Line, r.RawTargetText)
				}
			}
		})
	}
}
