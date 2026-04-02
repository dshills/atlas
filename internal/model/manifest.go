package model

// Manifest represents the .atlas/manifest.json file.
type Manifest struct {
	RepoRoot         string `json:"repo_root"`
	SchemaVersion    int    `json:"schema_version"`
	GeneratorVersion string `json:"generator_version"`
}
