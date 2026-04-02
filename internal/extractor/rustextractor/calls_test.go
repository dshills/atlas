package rustextractor

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
			content:   "fn main() {\n    process_data(x);\n}\n",
			wantTexts: []string{"process_data"},
		},
		{
			name:      "path call",
			content:   "fn main() {\n    std::fs::read_to_string(path);\n}\n",
			wantTexts: []string{"std::fs::read_to_string"},
		},
		{
			name:      "selector call",
			content:   "fn main() {\n    client.send();\n}\n",
			wantTexts: []string{"client.send"},
		},
		{
			name:      "macro call",
			content:   "fn main() {\n    custom_macro!(args);\n}\n",
			wantTexts: []string{"custom_macro!"},
		},
		{
			name:     "keyword not extracted",
			content:  "fn main() {\n    if condition {\n    }\n}\n",
			wantNone: []string{"if"},
		},
		{
			name:     "excluded macro not extracted",
			content:  "fn main() {\n    println!(\"hello\");\n}\n",
			wantNone: []string{"println!"},
		},
		{
			name:     "comment excluded",
			content:  "fn main() {\n    // process_data(x)\n}\n",
			wantNone: []string{"process_data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.content, "\n")
			codeLines := commentfilter.LineFilter(tt.content, "rust")
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
