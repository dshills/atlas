package diag

import "testing"

func TestCollector(t *testing.T) {
	c := NewCollector()

	c.AddInfo(CodeFileMissing, "test info")
	c.AddWarning(CodeFileMissing, "test warning")
	c.AddError(CodeParseError, "test error")

	if len(c.All()) != 3 {
		t.Errorf("expected 3 diagnostics, got %d", len(c.All()))
	}
	if c.ErrorCount() != 1 {
		t.Errorf("expected 1 error, got %d", c.ErrorCount())
	}
	if c.WarningCount() != 1 {
		t.Errorf("expected 1 warning, got %d", c.WarningCount())
	}
	if !c.HasErrors() {
		t.Error("expected HasErrors to be true")
	}
}

func TestCollectorNoErrors(t *testing.T) {
	c := NewCollector()
	c.AddInfo(CodeFileMissing, "just info")
	if c.HasErrors() {
		t.Error("expected no errors")
	}
}
