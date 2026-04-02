package pyextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractCalls(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCalls []string
		wantNone  []string
	}{
		{
			name:      "direct function call",
			content:   "result = process_data(x)\n",
			wantCalls: []string{"process_data"},
		},
		{
			name:      "selector call",
			content:   "data = service.get_data()\n",
			wantCalls: []string{"service.get_data"},
		},
		{
			name:     "if keyword not a call",
			content:  "if condition:\n    pass\n",
			wantNone: []string{"if"},
		},
		{
			name:     "def keyword not a call",
			content:  "def my_func():\n    pass\n",
			wantNone: []string{"def"},
		},
		{
			name:     "builtin excluded",
			content:  "print(\"hello\")\n",
			wantNone: []string{"print"},
		},
		{
			name:     "comment excluded",
			content:  "# process_data(x)\n",
			wantNone: []string{"process_data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines(tt.content)
			codeLines := commentfilter.LineFilter(tt.content, "python")
			refs := extractCalls(tt.content, lines, codeLines, nil, "test_module")

			callTargets := map[string]bool{}
			for _, r := range refs {
				if r.ReferenceKind != "calls" {
					t.Errorf("unexpected reference kind %q", r.ReferenceKind)
				}
				if r.Confidence != "heuristic" {
					t.Errorf("unexpected confidence %q", r.Confidence)
				}
				callTargets[r.RawTargetText] = true
			}

			for _, want := range tt.wantCalls {
				if !callTargets[want] {
					t.Errorf("expected call to %q not found in %v", want, callTargets)
				}
			}
			for _, unwanted := range tt.wantNone {
				if callTargets[unwanted] {
					t.Errorf("unexpected call to %q found", unwanted)
				}
			}
		})
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Ensure extractCalls signature matches expected usage with symbols param.
var _ = func() []extractor.ReferenceRecord {
	return extractCalls("", nil, nil, []extractor.SymbolRecord{}, "mod")
}
