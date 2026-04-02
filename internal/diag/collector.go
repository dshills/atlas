package diag

import "sync"

// Collector accumulates diagnostics during an index run.
type Collector struct {
	mu          sync.Mutex
	diagnostics []Diagnostic
}

// NewCollector creates a new diagnostic collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Add appends a diagnostic.
func (c *Collector) Add(d Diagnostic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diagnostics = append(c.diagnostics, d)
}

// AddError adds an error diagnostic with the given code and message.
func (c *Collector) AddError(code, message string) {
	c.Add(Diagnostic{Severity: SeverityError, Code: code, Message: message})
}

// AddWarning adds a warning diagnostic.
func (c *Collector) AddWarning(code, message string) {
	c.Add(Diagnostic{Severity: SeverityWarning, Code: code, Message: message})
}

// AddInfo adds an info diagnostic.
func (c *Collector) AddInfo(code, message string) {
	c.Add(Diagnostic{Severity: SeverityInfo, Code: code, Message: message})
}

// All returns all collected diagnostics.
func (c *Collector) All() []Diagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Diagnostic, len(c.diagnostics))
	copy(result, c.diagnostics)
	return result
}

// ErrorCount returns the number of error and fatal diagnostics.
func (c *Collector) ErrorCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, d := range c.diagnostics {
		if d.Severity == SeverityError || d.Severity == SeverityFatal {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning diagnostics.
func (c *Collector) WarningCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, d := range c.diagnostics {
		if d.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// HasErrors returns true if any error or fatal diagnostics were collected.
func (c *Collector) HasErrors() bool {
	return c.ErrorCount() > 0
}
