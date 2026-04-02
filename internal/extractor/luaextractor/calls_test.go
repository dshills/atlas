package luaextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

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
			name:      "colon call",
			content:   "obj:method(arg)\n",
			wantCalls: []string{"obj:method"},
		},
		{
			name:     "keyword not a call",
			content:  "if condition then\n    return true\nend\n",
			wantNone: []string{"if", "then", "return", "end"},
		},
		{
			name:     "function keyword not a call",
			content:  "local function foo()\nend\n",
			wantNone: []string{"function"},
		},
		{
			name:     "comment excluded",
			content:  "-- process_data(x)\n",
			wantNone: []string{"process_data"},
		},
		{
			name:      "multiple colon calls",
			content:   "self:init()\nplayer:update(dt)\n",
			wantCalls: []string{"self:init", "player:update"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines(tt.content)
			codeLines := commentfilter.LineFilter(tt.content, "lua")
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

func TestExtractCalls_Dedup(t *testing.T) {
	content := "foo(1) foo(2)\n"
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "lua")
	refs := extractCalls(content, lines, codeLines, nil, "mod")

	count := 0
	for _, r := range refs {
		if r.RawTargetText == "foo" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduped call to 'foo', got %d", count)
	}
}

// Ensure extractCalls signature matches expected usage with symbols param.
var _ = func() []extractor.ReferenceRecord {
	return extractCalls("", nil, nil, []extractor.SymbolRecord{}, "mod")
}
