package diag

// Severity levels for diagnostics.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
	SeverityFatal   = "fatal"
)

// Diagnostic codes per Section 22.4.
const (
	CodeParseError        = "PARSE_ERROR"
	CodeExtractPartial    = "EXTRACT_PARTIAL"
	CodeSummaryFailed     = "SUMMARY_FAILED"
	CodeConfigInvalid     = "CONFIG_INVALID"
	CodeSchemaMismatch    = "SCHEMA_MISMATCH"
	CodeOrphanedReference = "ORPHANED_REFERENCE"
	CodeFileMissing       = "FILE_MISSING"
	CodeUnsupportedLang   = "UNSUPPORTED_LANGUAGE"
)

// Diagnostic represents a single diagnostic entry.
type Diagnostic struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	FileID      int64  `json:"file_id,omitempty"`
	Line        int    `json:"line,omitempty"`
	ColumnStart int    `json:"column_start,omitempty"`
	ColumnEnd   int    `json:"column_end,omitempty"`
	DetailsJSON string `json:"details_json,omitempty"`
}
