package config

const (
	DefaultStorageDir    = ".atlas"
	DefaultDBFile        = "atlas.db"
	DefaultConfigFile    = "config.yaml"
	DefaultManifestFile  = "manifest.json"
	DefaultMaxFileSize   = int64(1 << 20) // 1 MiB
	DefaultSummaryMaxLen = 500
)

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Version:    1,
		StorageDir: DefaultStorageDir,
		Include:    []string{"**/*.go", "**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx", "**/*.py", "**/*.pyi", "**/*.rs"},
		Exclude:    []string{"vendor/**", "node_modules/**", ".git/**", "testdata/**", "__pycache__/**", "target/**", ".venv/**", "venv/**"},
		Languages: LanguageConfig{
			Go:         true,
			TypeScript: true,
			JavaScript: true,
			Python:     true,
			Rust:       true,
		},
		Indexing: IndexingConfig{
			MaxFileSizeBytes: DefaultMaxFileSize,
		},
		Summaries: SummaryConfig{
			Enabled: true,
			File:    true,
			Package: true,
			Symbol:  true,
		},
		Output: OutputConfig{
			DefaultFormat: "text",
		},
	}
}

// DefaultConfigYAML returns the default config as YAML for writing to config.yaml.
func DefaultConfigYAML() string {
	return `# Atlas configuration
version: 1
include:
  - "**/*.go"
  - "**/*.ts"
  - "**/*.tsx"
  - "**/*.js"
  - "**/*.jsx"
  - "**/*.py"
  - "**/*.pyi"
  - "**/*.rs"
exclude:
  - "vendor/**"
  - "node_modules/**"
  - ".git/**"
  - "testdata/**"
  - "__pycache__/**"
  - "target/**"
  - ".venv/**"
  - "venv/**"
languages:
  go: true
  typescript: true
  javascript: true
  python: true
  rust: true
indexing:
  max_file_size_bytes: 1048576
summaries:
  enabled: true
  file: true
  package: true
  symbol: true
output:
  default_format: text
`
}
