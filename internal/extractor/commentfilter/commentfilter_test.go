package commentfilter

import (
	"strings"
	"testing"
)

func TestLineFilter_TypeScript_SingleLineComments(t *testing.T) {
	content := `const x = 1;
// this is a comment
const y = 2;
   // indented comment
const z = x + y; // trailing comment`

	result := LineFilter(content, "typescript")
	expected := []bool{true, false, true, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_TypeScript_BlockComments(t *testing.T) {
	content := `const x = 1;
/* block comment
   spanning multiple
   lines */
const y = 2;`

	result := LineFilter(content, "typescript")
	expected := []bool{true, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_TypeScript_BlockCommentSingleLine(t *testing.T) {
	content := `const x = 1;
/* single line block */
const y = 2;`

	result := LineFilter(content, "typescript")
	expected := []bool{true, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_TypeScript_CodeBeforeBlock(t *testing.T) {
	content := `const x = 1; /* starts block
still in block */
const y = 2;`

	result := LineFilter(content, "typescript")
	// Line 0 has code before /*, so it's code.
	expected := []bool{true, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_JavaScript(t *testing.T) {
	content := `// comment
const x = 1;`

	result := LineFilter(content, "javascript")
	expected := []bool{false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Rust_SingleLineComments(t *testing.T) {
	content := `let x = 1;
// comment
/// doc comment
//! inner doc
let y = 2;`

	result := LineFilter(content, "rust")
	expected := []bool{true, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Rust_NestedBlockComments(t *testing.T) {
	content := `let x = 1;
/* outer
  /* inner */
  still outer
*/
let y = 2;`

	result := LineFilter(content, "rust")
	expected := []bool{true, false, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Rust_NonNestedVsNested(t *testing.T) {
	// In TypeScript (non-nestable), the inner */ closes the block.
	tsContent := `/* outer
  /* inner */
code here
*/
more code`

	tsResult := LineFilter(tsContent, "typescript")
	// Line 0: block opens (depth 0->1), non-code
	// Line 1: inner /* ignored (not nestable), */ closes, depth 1->0, non-code
	// Line 2: code
	// Line 3: looks like */ but we're at depth 0, so it's code with a stray */
	// Line 4: code
	tsExpected := []bool{false, false, true, true, true}
	assertLines(t, tsResult, tsExpected)

	// In Rust (nestable), the inner */ closes inner, outer */ closes outer.
	rustContent := `/* outer
  /* inner */
  still outer
*/
code here`

	rustResult := LineFilter(rustContent, "rust")
	rustExpected := []bool{false, false, false, false, true}
	assertLines(t, rustResult, rustExpected)
}

func TestLineFilter_Python_SingleLineComments(t *testing.T) {
	content := `x = 1
# comment
y = 2
   # indented comment
z = x + y  # trailing comment`

	result := LineFilter(content, "python")
	expected := []bool{true, false, true, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Python_TripleDoubleQuote(t *testing.T) {
	content := `x = 1
"""
docstring body
"""
y = 2`

	result := LineFilter(content, "python")
	expected := []bool{true, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Python_TripleSingleQuote(t *testing.T) {
	content := `x = 1
'''
docstring body
'''
y = 2`

	result := LineFilter(content, "python")
	expected := []bool{true, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Python_InlineTripleQuote(t *testing.T) {
	// Triple quote that opens and closes on the same line is a string literal, not a block.
	content := `x = """inline string"""
y = 2`

	result := LineFilter(content, "python")
	expected := []bool{true, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Python_CodeBeforeTripleQuote(t *testing.T) {
	content := `func("""
  block content
""")
y = 2`

	result := LineFilter(content, "python")
	// Line 0: code before """, marked as code
	expected := []bool{true, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_EmptyFile(t *testing.T) {
	for _, lang := range []string{"typescript", "python", "rust"} {
		result := LineFilter("", lang)
		if len(result) != 1 {
			t.Errorf("lang=%s: expected 1 line for empty content, got %d", lang, len(result))
			continue
		}
		if !result[0] {
			t.Errorf("lang=%s: empty line should be code", lang)
		}
	}
}

func TestLineFilter_AllComments(t *testing.T) {
	content := `// line 1
// line 2
// line 3`

	result := LineFilter(content, "typescript")
	for i, v := range result {
		if v {
			t.Errorf("line %d should be comment", i)
		}
	}
}

func TestLineFilter_BlockCommentSpansEntireFile(t *testing.T) {
	content := `/*
everything
is
a
comment
*/`

	result := LineFilter(content, "typescript")
	for i, v := range result {
		if v {
			t.Errorf("line %d should be comment", i)
		}
	}
}

func TestLineFilter_TypeScript_CommentDelimInString(t *testing.T) {
	// Known limitation: /* inside a string is misinterpreted.
	// This test documents the behavior rather than asserting correctness.
	content := `const s = "/* not a comment */";
const y = 2;`

	result := LineFilter(content, "typescript")
	// Ideally both would be true, but the filter cannot distinguish string
	// contents from real comments. Line 0 has code before /*, so it's code.
	// Line 1 may or may not be affected depending on whether */ closes the block.
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	// Line 0 has code before the /*, so it's marked as code.
	if !result[0] {
		t.Error("line 0 should be code (code before /*)")
	}
}

func TestLineFilter_Python_TripleQuoteInsideString(t *testing.T) {
	// Known limitation: triple quotes inside regular strings may be misinterpreted.
	// This test documents the expected (imperfect) behavior.
	content := `x = "has ''' inside"
y = 2`

	result := LineFilter(content, "python")
	// The filter sees ''' and may enter block mode. This is a known limitation.
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
}

func TestLineFilter_UnknownLanguage(t *testing.T) {
	content := `// this looks like a comment
but everything is code`

	result := LineFilter(content, "unknown")
	for i, v := range result {
		if !v {
			t.Errorf("line %d should be code for unknown language", i)
		}
	}
}

func TestLineFilter_Python_MixedTripleQuotes(t *testing.T) {
	content := strings.Join([]string{
		`x = 1`,
		`"""`,
		`docstring with ''' inside`,
		`"""`,
		`y = 2`,
	}, "\n")

	result := LineFilter(content, "python")
	expected := []bool{true, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Java_SingleLineAndBlock(t *testing.T) {
	content := `int x = 1;
// single line comment
int y = 2;
/* block comment
   spanning lines */
int z = 3;`

	result := LineFilter(content, "java")
	expected := []bool{true, false, true, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_CSharp_SingleLineAndBlock(t *testing.T) {
	content := `int x = 1;
// single line comment
int y = 2;
/* block comment
   spanning lines */
int z = 3;`

	result := LineFilter(content, "csharp")
	expected := []bool{true, false, true, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Swift_NestedBlockComments(t *testing.T) {
	content := `let x = 1
/* outer
  /* inner */
  still outer
*/
let y = 2`

	result := LineFilter(content, "swift")
	expected := []bool{true, false, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Lua_SingleLineComments(t *testing.T) {
	content := `local x = 1
-- this is a comment
local y = 2
   -- indented comment
local z = x + y`

	result := LineFilter(content, "lua")
	expected := []bool{true, false, true, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Lua_BlockComments(t *testing.T) {
	content := `local x = 1
--[[
block comment
spanning lines
]]
local y = 2`

	result := LineFilter(content, "lua")
	expected := []bool{true, false, false, false, false, true}
	assertLines(t, result, expected)
}

func TestLineFilter_Lua_MixedComments(t *testing.T) {
	content := strings.Join([]string{
		`local x = 1`,
		`-- single line comment`,
		`local y = 2`,
		`--[[`,
		`block comment`,
		`]]`,
		`local z = 3`,
		`-- another single line`,
	}, "\n")

	result := LineFilter(content, "lua")
	expected := []bool{true, false, true, false, false, false, true, false}
	assertLines(t, result, expected)
}

func assertLines(t *testing.T, got, expected []bool) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("length mismatch: got %d, expected %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			label := "code"
			if !expected[i] {
				label = "comment"
			}
			t.Errorf("line %d: expected %s (got %v)", i, label, got[i])
		}
	}
}
