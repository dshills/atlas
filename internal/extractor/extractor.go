package extractor

import "context"

// Extractor defines the interface for language-specific code extraction (Section 13.2).
type Extractor interface {
	Language() string
	Supports(path string) bool
	SupportedKinds() []string
	Extract(ctx context.Context, req ExtractRequest) (*ExtractResult, error)
}

// ExtractRequest contains the input for extraction.
type ExtractRequest struct {
	FilePath   string // relative path
	AbsPath    string // absolute path
	Content    []byte
	RepoRoot   string
	ModulePath string // go module path from go.mod, if known
}

// ExtractResult contains the output of extraction.
type ExtractResult struct {
	File        *FileRecord
	Package     *PackageRecord
	Symbols     []SymbolRecord
	References  []ReferenceRecord
	Artifacts   []ArtifactRecord
	Diagnostics []DiagnosticRecord
}

// FileRecord holds extracted file metadata.
type FileRecord struct {
	ParseStatus string // "ok", "error", "partial"
}

// PackageRecord holds extracted package info.
type PackageRecord struct {
	Name          string
	ImportPath    string
	DirectoryPath string
	Language      string
}

// SymbolRecord holds an extracted symbol.
type SymbolRecord struct {
	Name           string
	QualifiedName  string
	SymbolKind     string
	Visibility     string
	ParentSymbolID string // qualified name of parent, for resolution
	Signature      string
	DocComment     string
	StartLine      int
	EndLine        int
	StableID       string
}

// ReferenceRecord holds an extracted reference.
type ReferenceRecord struct {
	FromSymbolName string // qualified name of source
	ToSymbolName   string // qualified name or raw target
	ReferenceKind  string
	Confidence     string
	Line           int
	ColumnStart    int
	ColumnEnd      int
	RawTargetText  string
}

// ArtifactRecord holds an extracted artifact.
type ArtifactRecord struct {
	ArtifactKind string
	Name         string
	SymbolName   string // qualified name of associated symbol
	DataJSON     string
	Confidence   string
}

// DiagnosticRecord holds an extraction diagnostic.
type DiagnosticRecord struct {
	Severity string
	Code     string
	Message  string
	Line     int
}
