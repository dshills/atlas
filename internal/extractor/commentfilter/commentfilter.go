// Package commentfilter provides a line-level filter that marks source lines
// as code or comment for use by regex-based extractors.
//
// Known limitations (accepted by design):
//   - Comment delimiters inside string literals (e.g., "/* not a comment */")
//     are not distinguished from real comments. This may cause false non-code
//     marking in rare cases.
//   - Python triple-quote blocks used as string literals (not docstrings) are
//     treated the same as docstrings.
//
// These trade-offs are intentional: the filter eliminates the vast majority of
// false positives from regex-based extraction without requiring a full parser.
package commentfilter

import "strings"

// LineFilter returns a slice of booleans, one per line, where true means the
// line is code and false means the line is inside a comment or docstring block.
// Supported lang values: "typescript", "javascript", "python", "rust".
// Lines containing both code and a trailing comment are marked as code.
func LineFilter(content string, lang string) []bool {
	lines := strings.Split(content, "\n")
	result := make([]bool, len(lines))

	switch lang {
	case "typescript", "javascript":
		filterCStyle(lines, result, false)
	case "rust":
		filterCStyle(lines, result, true)
	case "python":
		filterPython(lines, result)
	default:
		// Unknown language: treat all lines as code.
		for i := range result {
			result[i] = true
		}
	}

	return result
}

// filterCStyle handles // single-line and /* */ block comments.
// If nestable is true, block comments can nest (Rust).
// Note: does not track string literal boundaries, so "/*" inside a string
// will be treated as a block comment opener.
func filterCStyle(lines []string, result []bool, nestable bool) {
	depth := 0 // block comment nesting depth

	for i, line := range lines {
		if depth > 0 {
			// We're inside a block comment — check if this line closes it.
			result[i] = false
			depth = updateBlockDepth(line, depth, nestable)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			result[i] = false
			continue
		}

		// Check if line opens a block comment.
		if idx := strings.Index(trimmed, "/*"); idx >= 0 {
			// If there's code before the /*, mark as code.
			if idx > 0 {
				result[i] = true
			} else {
				result[i] = false
			}
			depth = updateBlockDepth(line, depth, nestable)
			continue
		}

		result[i] = true
	}
}

// updateBlockDepth scans a line for /* and */ and returns the updated depth.
func updateBlockDepth(line string, depth int, nestable bool) int {
	i := 0
	for i < len(line)-1 {
		if line[i] == '/' && line[i+1] == '*' {
			if nestable || depth == 0 {
				depth++
			}
			i += 2
			continue
		}
		if line[i] == '*' && line[i+1] == '/' {
			if depth > 0 {
				depth--
			}
			i += 2
			continue
		}
		i++
	}
	return depth
}

// filterPython handles # single-line comments and triple-quote blocks.
func filterPython(lines []string, result []bool) {
	inBlock := false
	blockDelim := "" // `"""` or `'''`

	for i, line := range lines {
		if inBlock {
			// Check if this line closes the block.
			if strings.Contains(line, blockDelim) {
				inBlock = false
			}
			result[i] = false
			continue
		}

		trimmed := strings.TrimSpace(line)

		// Single-line comment.
		if strings.HasPrefix(trimmed, "#") {
			result[i] = false
			continue
		}

		// Check for triple-quote block opening.
		if delim, opens := opensTripleQuote(trimmed); opens {
			// Check if the block also closes on the same line.
			// Count occurrences: if odd, block stays open; if even, it closes.
			rest := trimmed[strings.Index(trimmed, delim)+3:]
			if strings.Contains(rest, delim) {
				// Opens and closes on same line — treat as code (it's a string literal).
				result[i] = true
			} else {
				inBlock = true
				blockDelim = delim
				// If code precedes the triple-quote, mark as code.
				if strings.Index(trimmed, delim) > 0 {
					result[i] = true
				} else {
					result[i] = false
				}
			}
			continue
		}

		result[i] = true
	}
}

// opensTripleQuote checks if a trimmed line contains a triple-quote opener.
// Returns the delimiter and true if found.
func opensTripleQuote(trimmed string) (string, bool) {
	dq := strings.Index(trimmed, `"""`)
	sq := strings.Index(trimmed, `'''`)

	switch {
	case dq >= 0 && (sq < 0 || dq < sq):
		return `"""`, true
	case sq >= 0:
		return `'''`, true
	default:
		return "", false
	}
}
