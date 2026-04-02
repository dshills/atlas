package output

import (
	"encoding/json"
	"fmt"
)

// WriteAgent writes v as compact JSON with no indentation (agent mode).
func (f *Formatter) WriteAgent(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding agent JSON: %w", err)
	}
	_, err = fmt.Fprintln(f.W, string(data))
	return err
}
