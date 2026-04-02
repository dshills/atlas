package output

import "io"

// Mode represents the output format.
type Mode string

const (
	ModeText  Mode = "text"
	ModeJSON  Mode = "json"
	ModeAgent Mode = "agent"
)

// Formatter writes structured output in different modes.
type Formatter struct {
	Mode Mode
	W    io.Writer
}

// New creates a Formatter for the given mode.
func New(w io.Writer, mode Mode) *Formatter {
	return &Formatter{Mode: mode, W: w}
}

// Write outputs v using the formatter's current mode.
func (f *Formatter) Write(v any) error {
	switch f.Mode {
	case ModeAgent:
		return f.WriteAgent(v)
	case ModeJSON:
		return f.WriteJSON(v)
	default:
		return f.WriteJSON(v) // text mode for structured data falls back to JSON
	}
}
