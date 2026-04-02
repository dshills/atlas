package swiftextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractCalls(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantTexts []string
		wantNone  []string
	}{
		{
			name:      "direct function call",
			content:   "func main() {\n    processData(x)\n}\n",
			wantTexts: []string{"processData"},
		},
		{
			name:      "selector call",
			content:   "func main() {\n    manager.fetchData()\n}\n",
			wantTexts: []string{"manager.fetchData"},
		},
		{
			name:     "keyword not extracted",
			content:  "func main() {\n    if condition {\n    }\n}\n",
			wantNone: []string{"if"},
		},
		{
			name:     "comment excluded",
			content:  "func main() {\n    // processData(x)\n}\n",
			wantNone: []string{"processData"},
		},
		{
			name:     "swift keywords excluded",
			content:  "func main() {\n    guard let x = y else { return }\n}\n",
			wantNone: []string{"guard", "return"},
		},
		{
			name:      "multiple calls on different lines",
			content:   "func main() {\n    setup()\n    teardown()\n}\n",
			wantTexts: []string{"setup", "teardown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.content, "\n")
			codeLines := commentfilter.LineFilter(tt.content, "swift")
			refs := extractCalls(tt.content, lines, codeLines, []extractor.SymbolRecord{}, "test")

			rawTexts := make(map[string]bool)
			for _, r := range refs {
				rawTexts[r.RawTargetText] = true
				if r.ReferenceKind != "calls" {
					t.Errorf("expected ReferenceKind 'calls', got %q", r.ReferenceKind)
				}
				if r.Confidence != "heuristic" {
					t.Errorf("expected Confidence 'heuristic', got %q", r.Confidence)
				}
			}

			for _, want := range tt.wantTexts {
				if !rawTexts[want] {
					t.Errorf("expected call ref with RawTargetText %q, got refs: %v", want, refTexts(refs))
				}
			}

			for _, noWant := range tt.wantNone {
				if rawTexts[noWant] {
					t.Errorf("did not expect call ref with RawTargetText %q, but found it", noWant)
				}
			}
		})
	}
}

func refTexts(refs []extractor.ReferenceRecord) []string {
	var texts []string
	for _, r := range refs {
		texts = append(texts, r.RawTargetText)
	}
	return texts
}
