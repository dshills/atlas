package store

import (
	"database/sql"
	"encoding/json"
)

// FileSummaryRow represents a row in the file_summaries table.
type FileSummaryRow struct {
	FileID            int64    `json:"fileId"`
	SummaryText       string   `json:"summaryText"`
	Responsibilities  []string `json:"responsibilities"`
	KeySymbols        []string `json:"keySymbols"`
	Invariants        []string `json:"invariants"`
	SideEffects       []string `json:"sideEffects"`
	Dependencies      []string `json:"dependencies"`
	PublicAPI         []string `json:"publicApi"`
	Risks             []string `json:"risks"`
	RelatedArtifacts  []string `json:"relatedArtifacts"`
	GeneratedFromHash string   `json:"generatedFromHash"`
	GeneratorVersion  string   `json:"generatorVersion"`
}

// PackageSummaryRow represents a row in the package_summaries table.
type PackageSummaryRow struct {
	PackageID             int64    `json:"packageId"`
	SummaryText           string   `json:"summaryText"`
	MajorResponsibilities []string `json:"majorResponsibilities"`
	ExportedSurface       []string `json:"exportedSurface"`
	InternalCollaborators []string `json:"internalCollaborators"`
	ExternalDependencies  []string `json:"externalDependencies"`
	NotableInvariants     []string `json:"notableInvariants"`
	Risks                 []string `json:"risks"`
	GeneratedFromHash     string   `json:"generatedFromHash"`
	GeneratorVersion      string   `json:"generatorVersion"`
}

// SymbolSummaryRow represents a row in the symbol_summaries table.
type SymbolSummaryRow struct {
	SymbolID          int64    `json:"symbolId"`
	SummaryText       string   `json:"summaryText"`
	Intent            []string `json:"intent"`
	Inputs            []string `json:"inputs"`
	Outputs           []string `json:"outputs"`
	SideEffects       []string `json:"sideEffects"`
	FailureModes      []string `json:"failureModes"`
	Invariants        []string `json:"invariants"`
	RelatedSymbols    []string `json:"relatedSymbols"`
	GeneratedFromHash string   `json:"generatedFromHash"`
	GeneratorVersion  string   `json:"generatorVersion"`
}

func parseJSONArray(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(s.String), &result); err != nil {
		return []string{}
	}
	return result
}

// GetFileSummary retrieves a file summary by file ID.
func (s *Store) GetFileSummary(fileID int64) (*FileSummaryRow, error) {
	row := &FileSummaryRow{FileID: fileID}
	var resp, keys, inv, se, deps, api, risks, arts sql.NullString
	err := s.DB.QueryRow(`SELECT summary_text, responsibilities_json, key_symbols_json, invariants_json,
		side_effects_json, dependencies_json, public_api_json, risks_json, related_artifacts_json,
		generated_from_hash, generator_version
		FROM file_summaries WHERE file_id = ?`, fileID).
		Scan(&row.SummaryText, &resp, &keys, &inv, &se, &deps, &api, &risks, &arts,
			&row.GeneratedFromHash, &row.GeneratorVersion)
	if err != nil {
		return nil, err
	}
	row.Responsibilities = parseJSONArray(resp)
	row.KeySymbols = parseJSONArray(keys)
	row.Invariants = parseJSONArray(inv)
	row.SideEffects = parseJSONArray(se)
	row.Dependencies = parseJSONArray(deps)
	row.PublicAPI = parseJSONArray(api)
	row.Risks = parseJSONArray(risks)
	row.RelatedArtifacts = parseJSONArray(arts)
	return row, nil
}

// GetPackageSummary retrieves a package summary by package ID.
func (s *Store) GetPackageSummary(packageID int64) (*PackageSummaryRow, error) {
	row := &PackageSummaryRow{PackageID: packageID}
	var resp, exp, collab, ext, inv, risks sql.NullString
	err := s.DB.QueryRow(`SELECT summary_text, major_responsibilities_json, exported_surface_json,
		internal_collaborators_json, external_dependencies_json, notable_invariants_json, risks_json,
		generated_from_hash, generator_version
		FROM package_summaries WHERE package_id = ?`, packageID).
		Scan(&row.SummaryText, &resp, &exp, &collab, &ext, &inv, &risks,
			&row.GeneratedFromHash, &row.GeneratorVersion)
	if err != nil {
		return nil, err
	}
	row.MajorResponsibilities = parseJSONArray(resp)
	row.ExportedSurface = parseJSONArray(exp)
	row.InternalCollaborators = parseJSONArray(collab)
	row.ExternalDependencies = parseJSONArray(ext)
	row.NotableInvariants = parseJSONArray(inv)
	row.Risks = parseJSONArray(risks)
	return row, nil
}

// GetSymbolSummary retrieves a symbol summary by symbol ID.
func (s *Store) GetSymbolSummary(symbolID int64) (*SymbolSummaryRow, error) {
	row := &SymbolSummaryRow{SymbolID: symbolID}
	var intent, inputs, outputs, se, fm, inv, rel sql.NullString
	err := s.DB.QueryRow(`SELECT summary_text, intent_json, inputs_json, outputs_json,
		side_effects_json, failure_modes_json, invariants_json, related_symbols_json,
		generated_from_hash, generator_version
		FROM symbol_summaries WHERE symbol_id = ?`, symbolID).
		Scan(&row.SummaryText, &intent, &inputs, &outputs, &se, &fm, &inv, &rel,
			&row.GeneratedFromHash, &row.GeneratorVersion)
	if err != nil {
		return nil, err
	}
	row.Intent = parseJSONArray(intent)
	row.Inputs = parseJSONArray(inputs)
	row.Outputs = parseJSONArray(outputs)
	row.SideEffects = parseJSONArray(se)
	row.FailureModes = parseJSONArray(fm)
	row.Invariants = parseJSONArray(inv)
	row.RelatedSymbols = parseJSONArray(rel)
	return row, nil
}

// SummaryCount returns counts of file, package, and symbol summaries.
func (s *Store) SummaryCount() (fileSummaries, pkgSummaries, symSummaries int, err error) {
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM file_summaries`).Scan(&fileSummaries)
	if err != nil {
		return
	}
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM package_summaries`).Scan(&pkgSummaries)
	if err != nil {
		return
	}
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM symbol_summaries`).Scan(&symSummaries)
	return
}
