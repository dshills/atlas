// Package summary generates structured summaries of files, packages, and symbols
// from indexed data. Summaries are deterministic aggregations — not LLM-generated.
package summary

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	MaxSummaryText = 500
	MaxArrayEntry  = 100
)

// Generator produces summaries from indexed data.
type Generator struct {
	DB               *sql.DB
	GeneratorVersion string
}

// NewGenerator creates a new summary generator.
func NewGenerator(db *sql.DB, version string) *Generator {
	return &Generator{DB: db, GeneratorVersion: version}
}

// truncate ensures a string does not exceed max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// truncateEntries truncates each entry in a slice.
func truncateEntries(entries []string) []string {
	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = truncate(e, MaxArrayEntry)
	}
	return result
}

// toJSON marshals a value to JSON string, returns "[]" on error.
func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// GenerateFileSummary generates or regenerates a summary for a file.
func (g *Generator) GenerateFileSummary(fileID int64) error {
	// Get file info
	var path, contentHash, language string
	var pkgName sql.NullString
	err := g.DB.QueryRow(`SELECT path, content_hash, language, package_name FROM files WHERE id = ?`, fileID).
		Scan(&path, &contentHash, &language, &pkgName)
	if err != nil {
		return fmt.Errorf("file %d not found: %w", fileID, err)
	}

	// Gather symbols
	type symInfo struct {
		name, kind, visibility, signature, docComment string
	}
	rows, err := g.DB.Query(`SELECT name, symbol_kind, visibility, COALESCE(signature,''), COALESCE(doc_comment,'') FROM symbols WHERE file_id = ? ORDER BY start_line`, fileID)
	if err != nil {
		return fmt.Errorf("querying symbols: %w", err)
	}
	var symbols []symInfo
	for rows.Next() {
		var s symInfo
		if err := rows.Scan(&s.name, &s.kind, &s.visibility, &s.signature, &s.docComment); err != nil {
			_ = rows.Close()
			return err
		}
		symbols = append(symbols, s)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Build summary fields
	pkg := "unknown"
	if pkgName.Valid && pkgName.String != "" {
		pkg = pkgName.String
	}

	var responsibilities, keySymbols, publicAPI, sideEffects []string
	for _, s := range symbols {
		entry := fmt.Sprintf("%s %s", s.kind, s.name)
		if s.visibility == "exported" {
			keySymbols = append(keySymbols, s.name)
			apiEntry := s.name
			if s.signature != "" {
				apiEntry = s.signature
			}
			publicAPI = append(publicAPI, apiEntry)
		}
		responsibilities = append(responsibilities, entry)
	}

	// Gather dependencies (imports)
	depRows, err := g.DB.Query(`SELECT DISTINCT raw_target_text FROM "references" WHERE from_file_id = ? AND reference_kind = 'imports' AND raw_target_text IS NOT NULL`, fileID)
	if err != nil {
		return fmt.Errorf("querying dependencies: %w", err)
	}
	var dependencies []string
	for depRows.Next() {
		var dep string
		if err := depRows.Scan(&dep); err != nil {
			_ = depRows.Close()
			return err
		}
		dependencies = append(dependencies, dep)
	}
	_ = depRows.Close()

	// Gather related artifacts
	artRows, err := g.DB.Query(`SELECT artifact_kind, name FROM artifacts WHERE file_id = ?`, fileID)
	if err != nil {
		return fmt.Errorf("querying artifacts: %w", err)
	}
	var relatedArtifacts []string
	for artRows.Next() {
		var kind, name string
		if err := artRows.Scan(&kind, &name); err != nil {
			_ = artRows.Close()
			return err
		}
		relatedArtifacts = append(relatedArtifacts, fmt.Sprintf("%s:%s", kind, name))
	}
	_ = artRows.Close()

	// Check for side effects
	seRows, err := g.DB.Query(`SELECT DISTINCT reference_kind, raw_target_text FROM "references" WHERE from_file_id = ? AND reference_kind IN ('registers_route','uses_config','touches_table','invokes_external_api') AND raw_target_text IS NOT NULL`, fileID)
	if err != nil {
		return fmt.Errorf("querying side effects: %w", err)
	}
	for seRows.Next() {
		var kind, target string
		if err := seRows.Scan(&kind, &target); err != nil {
			_ = seRows.Close()
			return err
		}
		sideEffects = append(sideEffects, fmt.Sprintf("%s: %s", kind, target))
	}
	_ = seRows.Close()

	// Build summary text
	parts := []string{fmt.Sprintf("File %s in package %s.", path, pkg)}
	if len(symbols) > 0 {
		parts = append(parts, fmt.Sprintf("Contains %d symbols (%d exported).", len(symbols), len(keySymbols)))
	}
	if len(dependencies) > 0 {
		parts = append(parts, fmt.Sprintf("Imports %d packages.", len(dependencies)))
	}
	summaryText := truncate(strings.Join(parts, " "), MaxSummaryText)

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = g.DB.Exec(`INSERT OR REPLACE INTO file_summaries
		(file_id, summary_text, responsibilities_json, key_symbols_json, invariants_json, side_effects_json,
		 dependencies_json, public_api_json, risks_json, related_artifacts_json,
		 generated_from_hash, generator_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID, summaryText,
		toJSON(truncateEntries(responsibilities)),
		toJSON(truncateEntries(keySymbols)),
		toJSON([]string{}), // invariants - would need deeper analysis
		toJSON(truncateEntries(sideEffects)),
		toJSON(truncateEntries(dependencies)),
		toJSON(truncateEntries(publicAPI)),
		toJSON([]string{}), // risks
		toJSON(truncateEntries(relatedArtifacts)),
		contentHash, g.GeneratorVersion, now, now,
	)
	return err
}

// GeneratePackageSummary generates or regenerates a summary for a package.
func (g *Generator) GeneratePackageSummary(packageID int64) error {
	var pkgName, dirPath, language string
	var importPath sql.NullString
	err := g.DB.QueryRow(`SELECT name, import_path, directory_path, language FROM packages WHERE id = ?`, packageID).
		Scan(&pkgName, &importPath, &dirPath, &language)
	if err != nil {
		return fmt.Errorf("package %d not found: %w", packageID, err)
	}

	// Aggregate exported symbols across all files in the package
	rows, err := g.DB.Query(`SELECT s.name, s.symbol_kind, s.visibility, COALESCE(s.signature,'')
		FROM symbols s
		JOIN package_files pf ON s.file_id = pf.file_id
		WHERE pf.package_id = ?
		ORDER BY s.name`, packageID)
	if err != nil {
		return fmt.Errorf("querying package symbols: %w", err)
	}

	var exportedSurface, responsibilities []string
	var totalSymbols, exportedCount int
	for rows.Next() {
		var name, kind, vis, sig string
		if err := rows.Scan(&name, &kind, &vis, &sig); err != nil {
			_ = rows.Close()
			return err
		}
		totalSymbols++
		responsibilities = append(responsibilities, fmt.Sprintf("%s %s", kind, name))
		if vis == "exported" {
			exportedCount++
			entry := name
			if sig != "" {
				entry = sig
			}
			exportedSurface = append(exportedSurface, entry)
		}
	}
	_ = rows.Close()

	// Internal collaborators: other packages imported by files in this package
	collabRows, err := g.DB.Query(`SELECT DISTINCT r.raw_target_text
		FROM "references" r
		JOIN package_files pf ON r.from_file_id = pf.file_id
		WHERE pf.package_id = ? AND r.reference_kind = 'imports' AND r.raw_target_text IS NOT NULL`, packageID)
	if err != nil {
		return fmt.Errorf("querying collaborators: %w", err)
	}
	var internalCollabs, externalDeps []string
	for collabRows.Next() {
		var dep string
		if err := collabRows.Scan(&dep); err != nil {
			_ = collabRows.Close()
			return err
		}
		if strings.Contains(dep, ".") && !strings.HasPrefix(dep, "golang.org") && !strings.HasPrefix(dep, "google.golang.org") {
			// Heuristic: non-standard-library imports with dots are external
			externalDeps = append(externalDeps, dep)
		} else {
			internalCollabs = append(internalCollabs, dep)
		}
	}
	_ = collabRows.Close()

	// Compute composite hash from all files
	hashRow, err := g.DB.Query(`SELECT f.content_hash FROM files f JOIN package_files pf ON f.id = pf.file_id WHERE pf.package_id = ? ORDER BY f.path`, packageID)
	if err != nil {
		return fmt.Errorf("querying file hashes: %w", err)
	}
	var hashes []string
	for hashRow.Next() {
		var h string
		if err := hashRow.Scan(&h); err != nil {
			_ = hashRow.Close()
			return err
		}
		hashes = append(hashes, h)
	}
	_ = hashRow.Close()
	compositeHash := strings.Join(hashes, ":")

	summaryText := truncate(fmt.Sprintf("Package %s (%s). %d symbols (%d exported).", pkgName, language, totalSymbols, exportedCount), MaxSummaryText)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = g.DB.Exec(`INSERT OR REPLACE INTO package_summaries
		(package_id, summary_text, major_responsibilities_json, exported_surface_json,
		 internal_collaborators_json, external_dependencies_json, notable_invariants_json, risks_json,
		 generated_from_hash, generator_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		packageID, summaryText,
		toJSON(truncateEntries(responsibilities)),
		toJSON(truncateEntries(exportedSurface)),
		toJSON(truncateEntries(internalCollabs)),
		toJSON(truncateEntries(externalDeps)),
		toJSON([]string{}), // invariants
		toJSON([]string{}), // risks
		compositeHash, g.GeneratorVersion, now, now,
	)
	return err
}

// GenerateSymbolSummary generates or regenerates a summary for a symbol.
func (g *Generator) GenerateSymbolSummary(symbolID int64) error {
	var name, qualifiedName, kind, vis string
	var sig, doc sql.NullString
	var fileID int64
	err := g.DB.QueryRow(`SELECT s.name, s.qualified_name, s.symbol_kind, s.visibility, s.signature, s.doc_comment, s.file_id
		FROM symbols s WHERE s.id = ?`, symbolID).
		Scan(&name, &qualifiedName, &kind, &vis, &sig, &doc, &fileID)
	if err != nil {
		return fmt.Errorf("symbol %d not found: %w", symbolID, err)
	}

	// Get file hash for freshness
	var contentHash string
	err = g.DB.QueryRow(`SELECT content_hash FROM files WHERE id = ?`, fileID).Scan(&contentHash)
	if err != nil {
		return fmt.Errorf("file for symbol %d not found: %w", symbolID, err)
	}

	// Build intent from doc comment and kind
	var intent []string
	if doc.Valid && doc.String != "" {
		intent = append(intent, truncate(doc.String, MaxArrayEntry))
	}
	intent = append(intent, fmt.Sprintf("%s %s (%s)", vis, kind, name))

	// Extract inputs/outputs from signature
	var inputs, outputs []string
	if sig.Valid && sig.String != "" {
		inputs = append(inputs, truncate(sig.String, MaxArrayEntry))
	}

	// Find what this symbol calls
	callRows, err := g.DB.Query(`SELECT DISTINCT COALESCE(r.raw_target_text, '') FROM "references" r
		WHERE r.from_symbol_id = ? AND r.reference_kind = 'calls'`, symbolID)
	if err != nil {
		return fmt.Errorf("querying calls: %w", err)
	}
	var relatedSymbols []string
	for callRows.Next() {
		var target string
		if err := callRows.Scan(&target); err != nil {
			_ = callRows.Close()
			return err
		}
		if target != "" {
			relatedSymbols = append(relatedSymbols, target)
		}
	}
	_ = callRows.Close()

	// Find who calls this symbol
	callerRows, err := g.DB.Query(`SELECT DISTINCT COALESCE(s2.qualified_name, '') FROM "references" r
		JOIN symbols s2 ON r.from_symbol_id = s2.id
		WHERE r.to_symbol_id = ? AND r.reference_kind = 'calls'`, symbolID)
	if err != nil {
		return fmt.Errorf("querying callers: %w", err)
	}
	for callerRows.Next() {
		var caller string
		if err := callerRows.Scan(&caller); err != nil {
			_ = callerRows.Close()
			return err
		}
		if caller != "" {
			relatedSymbols = append(relatedSymbols, fmt.Sprintf("called by %s", caller))
		}
	}
	_ = callerRows.Close()

	summaryText := truncate(fmt.Sprintf("%s %s %s.", vis, kind, qualifiedName), MaxSummaryText)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = g.DB.Exec(`INSERT OR REPLACE INTO symbol_summaries
		(symbol_id, summary_text, intent_json, inputs_json, outputs_json,
		 side_effects_json, failure_modes_json, invariants_json, related_symbols_json,
		 generated_from_hash, generator_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		symbolID, summaryText,
		toJSON(truncateEntries(intent)),
		toJSON(truncateEntries(inputs)),
		toJSON(truncateEntries(outputs)),
		toJSON([]string{}), // side_effects
		toJSON([]string{}), // failure_modes
		toJSON([]string{}), // invariants
		toJSON(truncateEntries(relatedSymbols)),
		contentHash, g.GeneratorVersion, now, now,
	)
	return err
}

// GenerateAllForFile generates file and symbol summaries for a given file.
func (g *Generator) GenerateAllForFile(fileID int64) error {
	if err := g.GenerateFileSummary(fileID); err != nil {
		return err
	}

	rows, err := g.DB.Query(`SELECT id FROM symbols WHERE file_id = ?`, fileID)
	if err != nil {
		return err
	}
	var symIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		symIDs = append(symIDs, id)
	}
	_ = rows.Close()

	for _, id := range symIDs {
		if err := g.GenerateSymbolSummary(id); err != nil {
			return err
		}
	}
	return nil
}

// GenerateAllPackages generates summaries for all packages.
func (g *Generator) GenerateAllPackages() error {
	rows, err := g.DB.Query(`SELECT id FROM packages`)
	if err != nil {
		return err
	}
	var pkgIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		pkgIDs = append(pkgIDs, id)
	}
	_ = rows.Close()

	for _, id := range pkgIDs {
		if err := g.GeneratePackageSummary(id); err != nil {
			return err
		}
	}
	return nil
}
