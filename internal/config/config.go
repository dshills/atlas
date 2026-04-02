package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the full Atlas configuration from .atlas/config.yaml.
type Config struct {
	Version    int            `yaml:"version"    json:"version"`
	RepoRoot   string         `yaml:"repo_root"  json:"repo_root,omitempty"`
	StorageDir string         `yaml:"storage_dir" json:"storage_dir"`
	Include    []string       `yaml:"include"    json:"include"`
	Exclude    []string       `yaml:"exclude"    json:"exclude"`
	Languages  LanguageConfig `yaml:"languages"  json:"languages"`
	Indexing   IndexingConfig `yaml:"indexing"   json:"indexing"`
	Summaries  SummaryConfig  `yaml:"summaries"  json:"summaries"`
	Output     OutputConfig   `yaml:"output"     json:"output"`
}

// LanguageConfig controls which languages are enabled.
type LanguageConfig struct {
	Go         bool `yaml:"go"         json:"go"`
	TypeScript bool `yaml:"typescript" json:"typescript"`
	JavaScript bool `yaml:"javascript" json:"javascript"`
	Python     bool `yaml:"python"     json:"python"`
	Rust       bool `yaml:"rust"       json:"rust"`
	Java       bool `yaml:"java"       json:"java"`
	CSharp     bool `yaml:"csharp"     json:"csharp"`
	Swift      bool `yaml:"swift"      json:"swift"`
	Lua        bool `yaml:"lua"        json:"lua"`
}

// IndexingConfig controls indexing behavior.
type IndexingConfig struct {
	MaxFileSizeBytes int64 `yaml:"max_file_size_bytes" json:"max_file_size_bytes"`
}

// SummaryConfig controls summary generation.
type SummaryConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	File    bool `yaml:"file"    json:"file"`
	Package bool `yaml:"package" json:"package"`
	Symbol  bool `yaml:"symbol"  json:"symbol"`
}

// OutputConfig controls default output format.
type OutputConfig struct {
	DefaultFormat string `yaml:"default_format" json:"default_format"`
}

// Load reads configuration from the given YAML path, merging with defaults.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// LoadFromDir loads config from the .atlas/ directory under repoRoot.
func LoadFromDir(repoRoot string) (Config, error) {
	path := filepath.Join(repoRoot, DefaultStorageDir, DefaultConfigFile)
	cfg, err := Load(path)
	if err != nil {
		return cfg, err
	}
	if cfg.RepoRoot == "" {
		cfg.RepoRoot = repoRoot
	}
	return cfg, nil
}
