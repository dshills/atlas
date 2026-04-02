package csharpextractor

import (
	"testing"

	"github.com/dshills/atlas/internal/extractor"
	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractCalls_DirectCalls(t *testing.T) {
	content := `public void DoWork()
{
    initialize();
    processItems();
    cleanup();
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs := extractCalls(content, lines, codeLines, nil, "")

	names := make(map[string]bool)
	for _, r := range refs {
		if r.ReferenceKind == "calls" {
			names[r.RawTargetText] = true
		}
	}

	for _, expected := range []string{"initialize", "processItems", "cleanup"} {
		if !names[expected] {
			t.Errorf("expected call to %q not found", expected)
		}
	}
}

func TestExtractCalls_SelectorCalls(t *testing.T) {
	content := `public void DoWork()
{
    logger.LogInfo("starting");
    db.SaveChanges();
    service.Process();
}
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs := extractCalls(content, lines, codeLines, nil, "")

	names := make(map[string]bool)
	for _, r := range refs {
		if r.ReferenceKind == "calls" {
			names[r.RawTargetText] = true
		}
	}

	for _, expected := range []string{"logger.LogInfo", "db.SaveChanges", "service.Process"} {
		if !names[expected] {
			t.Errorf("expected call to %q not found", expected)
		}
	}
}

func TestExtractCalls_SkipKeywords(t *testing.T) {
	content := `if (true) { }
for (var i = 0; i < 10; i++) { }
while (running) { }
return;
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs := extractCalls(content, lines, codeLines, nil, "")

	for _, r := range refs {
		if csharpKeywords[r.RawTargetText] {
			t.Errorf("keyword %q should not appear as a call", r.RawTargetText)
		}
	}
}

func TestExtractCalls_Dedup(t *testing.T) {
	content := `doWork();
doWork();
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs := extractCalls(content, lines, codeLines, nil, "")

	// Two calls on different lines should both appear
	count := 0
	for _, r := range refs {
		if r.RawTargetText == "doWork" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 doWork calls (different lines), got %d", count)
	}
}

func TestExtractCalls_CommentExcluded(t *testing.T) {
	content := `// initialize();
processItems();
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs := extractCalls(content, lines, codeLines, nil, "")

	for _, r := range refs {
		if r.RawTargetText == "initialize" {
			t.Error("commented call should not appear")
		}
	}
}

func TestExtractCalls_Confidence(t *testing.T) {
	content := `doWork();
`
	lines := splitLines(content)
	codeLines := commentfilter.LineFilter(content, "csharp")

	refs := extractCalls(content, lines, codeLines, []extractor.SymbolRecord{}, "TestModule")

	if len(refs) == 0 {
		t.Fatal("expected at least 1 call ref")
	}
	if refs[0].Confidence != "heuristic" {
		t.Errorf("expected confidence 'heuristic', got %q", refs[0].Confidence)
	}
}
