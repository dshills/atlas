package output

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// WriteText writes a key-value map as aligned text.
func (f *Formatter) WriteText(pairs []KV) error {
	tw := tabwriter.NewWriter(f.W, 0, 4, 2, ' ', 0)
	for _, kv := range pairs {
		if _, err := fmt.Fprintf(tw, "%s:\t%s\n", kv.Key, kv.Value); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// WriteTextLines writes plain text lines.
func (f *Formatter) WriteTextLines(lines []string) error {
	_, err := fmt.Fprintln(f.W, strings.Join(lines, "\n"))
	return err
}

// KV is a key-value pair for text output.
type KV struct {
	Key   string
	Value string
}
