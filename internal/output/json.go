package output

import (
	"encoding/json"
	"fmt"
)

// WriteJSON writes v as indented JSON.
func (f *Formatter) WriteJSON(v any) error {
	enc := json.NewEncoder(f.W)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}
