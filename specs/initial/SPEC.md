# SPEC.md — Atlas

## 1. Product Overview

### 1.1 Name
Atlas

### 1.2 One-Sentence Purpose
Atlas is a Go CLI that builds and maintains a structural and semantic index of a source repository, stored in SQLite, so AI coding agents can query symbols, relationships, and summaries without rereading source files.

### 1.3 Product Vision
Traditional tags files were built to help humans jump to definitions. Atlas extends that idea into a queryable repository index that stores files, packages, symbols, relationships, higher-level artifacts, freshness state, and structured summaries.

Atlas enables coding agents and developers to answer structural and relationship questions about a codebase via CLI queries instead of scanning raw source files.

### 1.4 Core Value Proposition
Atlas must allow an agent to answer questions like:
- Where is this symbol defined?
- What implements this interface?
- Who calls this function?
- Which files likely need to be changed for this feature?
- Which tests are related to this package or symbol?
- What routes, config keys, or DB artifacts exist?
- What changed structurally since the last run?

without forcing the agent to repeatedly scan raw files or rely on brittle grep-based discovery.

---

## 2. Goals and Non-Goals

### 2.1 Primary Goals
1. Build a local index of repository structure and relationships that passes `atlas validate --strict` without errors.
2. Maintain that index incrementally as files change, updating changed files and their owned data (symbols, references, summaries), and marking cross-file references to deleted symbols as unresolved per Section 12.3.
3. Provide deterministic CLI queries that return results within the performance targets defined in Section 25.
4. Store facts in SQLite with content-hash-based freshness tracking per Section 12.
5. Expose structured JSON summaries and exports that allow agents to query repository facts without reading source files.

### 2.2 Secondary Goals
1. Provide a foundation for future tooling such as planning, test recommendation, documentation generation, and workflow observability.
2. Support multiple languages over time with a common storage schema and language-specific extractors.
3. Provide `doctor` and `validate` commands that detect integrity issues defined in Section 23, and `diagnostics` output that categorizes all indexing problems by severity.

### 2.3 Non-Goals
Atlas v1 is not intended to:
- compile, run, or test the project
- replace grep, ripgrep, or language servers
- become a full code intelligence server or IDE protocol implementation
- perform code generation or automatic code rewriting
- require cloud infrastructure or external services
- become a distributed or multi-user repository indexing platform
- rely on embeddings as the primary source of truth

---

## 3. Users and Primary Use Cases

### 3.1 Primary Users
- AI coding agents operating on local repositories
- Developers working in large or unfamiliar codebases
- Workflow tools that need repository structure and relationship data

### 3.2 Primary Use Cases

#### 3.2.1 Agent Repository Preparation
Before planning or coding, an agent runs Atlas to build a repository map and query likely targets.

#### 3.2.2 Incremental Development
After edits, Atlas updates only changed files and invalidates stale derived data.

#### 3.2.3 Repository Discovery
A developer uses Atlas to understand architecture, find symbols, locate routes, inspect config usage, and identify related tests.

#### 3.2.4 Workflow Integration
Other tools consume Atlas exports or CLI JSON output to scope planning, documentation, verification, or review.

---

## 4. Product Requirements

### 4.1 Core Functional Requirements
Atlas must:
1. Initialize repository-local storage and configuration.
2. Perform full repository indexing.
3. Perform incremental indexing based on file changes.
4. Extract supported-language structural facts.
5. Extract supported-language relationships.
6. Persist indexed facts in a durable local store.
7. Track freshness using content hashes.
8. Provide query commands for symbols, files, packages, artifacts, and relationships.
9. Provide structured summaries for files and packages.
10. Provide diagnostics, validation, and integrity inspection.
11. Provide JSON output suitable for agent use.

### 4.2 Reliability Requirements
If a file changes, stale derived data must not continue to appear as fresh. Partial indexing is acceptable. Silent corruption is not. Specifically: after any indexing run, `atlas validate` must report zero integrity errors for files whose `parse_status` is `ok`.

### 4.3 Operational Requirements
Atlas must run locally as a standalone CLI binary and work without network access.

---

## 5. Supported Repository Model

### 5.1 Repository Scope
Atlas v1 targets a single local repository root at a time.

### 5.2 Repository Root Detection
Atlas must detect the repository root using the following order:
1. explicit `--repo` flag
2. configured repo root in `.atlas/config.yaml`
3. Git root if available
4. current working directory

### 5.3 Storage Scope
Each indexed repository gets its own local `.atlas/` directory under the repository root by default.

---

## 6. Architectural Model

Atlas consists of four main layers:

### 6.1 Structural Layer
Stores facts about files, packages/modules, symbols, declarations, imports, tests, and entrypoints.

### 6.2 Relationship Layer
Stores edges such as:
- calls
- implements
- imports
- embeds
- tests
- registers_route
- uses_config
- touches_table
- migrates
- invokes_external_api

### 6.3 Semantic Layer
Stores compact, structured summaries of files, packages, and optionally selected symbols.

### 6.4 Freshness and State Layer
Stores content hashes, run metadata, generator versions, parser versions, diagnostics, and invalidation state.

---

## 7. CLI Requirements

## 7.1 Core Commands
Atlas must support these commands:

```bash
atlas init
atlas index
atlas reindex
atlas stale
atlas stats
atlas doctor
atlas validate
atlas version
atlas config
```

### 7.1.1 `atlas init`
Initializes `.atlas/` storage for a repository.

#### Behavior
- create `.atlas/` if missing
- create SQLite DB if missing and run schema migrations to the current version; fail with a non-zero exit code if migrations fail
- write default config if missing
- initialize schema metadata in `schema_meta`
- detect repo root
- print summary of initialized repository

#### Example
```bash
atlas init
atlas init --repo /path/to/repo
```

### 7.1.2 `atlas index`
Performs incremental indexing by default.

#### Behavior
- detect file changes via hashes and optional VCS scope
- scan files matching include globs, excluding files matching exclude globs
- skip files whose current content hash matches the stored content hash
- parse changed files
- rebuild changed-file symbols and references
- invalidate stale summaries
- persist new state transactionally
- record run stats and diagnostics

#### VCS Scope
The `--since` flag accepts a Git revision (e.g. `HEAD~1`, a commit SHA). Only Git is supported in v1. If the repository is not a Git repository or the revision is invalid, Atlas must exit with a non-zero status and an error diagnostic. When `--since` is provided, Atlas limits file change detection to files modified since the given revision according to `git diff`.

#### Example
```bash
atlas index
atlas index --json
atlas index --since HEAD~1
atlas index --strict
```

### 7.1.3 `atlas reindex`
Forces a full rebuild.

#### Behavior
- delete all rows from files, packages, package_files, symbols, references, artifacts, file_summaries, package_summaries, symbol_summaries, and diagnostics tables while preserving schema_meta and config
- rescan the repository and rebuild all files, packages, symbols, references, artifacts, and enabled summaries from scratch
- preserve configuration and schema metadata

#### Example
```bash
atlas reindex
atlas reindex --json
```

### 7.1.4 `atlas stale`
Reports stale derived data: file summaries, package summaries, and symbol summaries whose `generated_from_hash` does not match the current source `content_hash`.

#### Example
```bash
atlas stale
atlas stale --json
```

### 7.1.5 `atlas stats`
Shows current repository and index statistics.

#### Required Statistics
- total indexed files, broken down by language
- total packages
- total symbols, broken down by kind
- total references, broken down by kind
- total artifacts, broken down by kind
- stale summary count (file, package, symbol)
- last index run timestamp, mode, and status
- database file size
- active extractor capabilities per language

#### Example
```bash
atlas stats
atlas stats --json
```

### 7.1.6 `atlas doctor`
Performs health checks on the Atlas installation and index.

#### Example
```bash
atlas doctor
atlas doctor --json
```

### 7.1.7 `atlas validate`
Performs consistency validation against the database and current repository state.

#### Example
```bash
atlas validate
atlas validate --strict
```

### 7.1.8 `atlas version`
Prints the Atlas binary version, schema version, and Go runtime version.

#### Example
```bash
atlas version
atlas version --json
```

### 7.1.9 `atlas config`
Prints the current effective configuration, merging defaults with `.atlas/config.yaml`.

#### Example
```bash
atlas config
atlas config --json
```

---

## 7.2 Query Commands

### 7.2.1 Symbol Queries
```bash
atlas find symbol <name>
atlas find symbol <qualified-name>
```

#### Requirements
- resolution follows the order defined in Section 17.1
- exact match by simple name
- default matching is exact, case-sensitive against `name` and `qualified_name` fields
- `--fuzzy` flag enables case-insensitive substring matching
- optional filtering via flags: `--kind`, `--package`, `--file`, `--language`, `--visibility`
- output file path and line range
- support `--json` and `--agent`

### 7.2.2 File Queries
```bash
atlas find file <pattern>
```

#### Requirements
- substring matching on path
- `--exact` flag for exact path matching instead of substring
- support `--include` and `--exclude` flags accepting glob patterns (same doublestar syntax as config) to further filter results

### 7.2.3 Package Queries
```bash
atlas find package <name>
```

#### Requirements
- lookup by package/module name
- lookup by import path when the language defines import paths (e.g. Go module paths)

### 7.2.4 Artifact Queries
```bash
atlas find route <pattern>
atlas find config <key>
```

#### Requirements
- return extracted artifacts with source location and confidence level

---

## 7.3 Relationship Commands

### 7.3.1 Incoming Callers
```bash
atlas who-calls <symbol>
```

### 7.3.2 Outgoing Calls
```bash
atlas calls <symbol>
```

### 7.3.3 Interface Implementations
```bash
atlas implementations <interface>
```

### 7.3.4 Package Imports
```bash
atlas imports <package>
```

### 7.3.5 Tests for Target
```bash
atlas tests-for <target>
```

### 7.3.6 Artifact Touches
```bash
atlas touches <artifact-kind> <name>
```

#### Relationship Command Requirements
- resolve the subject symbol or artifact first
- return relationships ordered by the ranking rules defined in Section 17.2
- include source path and location when available
- expose confidence and whether the relationship is exact or heuristic

---

## 7.4 Summary Commands

### 7.4.1 File Summary
```bash
atlas summarize file <path>
```

### 7.4.2 Package Summary
```bash
atlas summarize package <name>
```

### 7.4.3 Symbol Summary
```bash
atlas summarize symbol <qualified-name>
```

#### Summary Output Requirements
Summaries must be structured JSON objects using the fields defined in Section 16. The `summary_text` field must not exceed 500 characters. Array fields (responsibilities, invariants, etc.) must contain concise phrases (under 100 characters each), not sentences or paragraphs.

---

## 7.5 Listing Commands

```bash
atlas list packages
atlas list routes
atlas list jobs
atlas list migrations
atlas list integrations
atlas list entrypoints
atlas list diagnostics
```

#### Command-to-Data Mapping
| Command | Data source |
|---------|-------------|
| `list packages` | `packages` table |
| `list routes` | `artifacts` where `artifact_kind = 'route'` |
| `list jobs` | `artifacts` where `artifact_kind = 'background_job'` |
| `list migrations` | `artifacts` where `artifact_kind = 'migration'` |
| `list integrations` | `artifacts` where `artifact_kind = 'external_service'` |
| `list entrypoints` | `symbols` where `symbol_kind = 'entrypoint'` |
| `list diagnostics` | `diagnostics` table |

#### Requirements
Each list command must support:
- human text output
- JSON output
- filtering by `--language` and `--name` (substring match)
- sorting by `--sort` flag accepting `name` (default) or `count`

---

## 7.6 Export Commands

```bash
atlas export summary
atlas export graph
atlas export symbols
atlas export packages
atlas export routes
atlas export diagnostics
```

#### Requirements
- default to JSON
- support writing to stdout or file
- stable machine-readable schema
- NDJSON support is not required for v1

---

## 8. Output Modes

### 8.1 Human Text Mode
Compact terminal-friendly output suitable for direct interactive use.

### 8.2 JSON Mode
Stable machine-readable output for automation and agent integration.

### 8.3 Agent Mode
JSON output with null and empty fields omitted, no whitespace formatting, suitable for direct inclusion in LLM prompts or tool responses.

#### Agent Mode Requirements
- no ANSI escape codes, box-drawing characters, or decorative formatting
- output JSON objects containing: the queried entity, its location, its freshness status, and its stable identifier
- omit fields that are null or empty
- string values outside of `summary_text` fields must not contain explanatory prose; use structured key-value data
- `summary_text` fields are permitted in Agent Mode output but must respect the 500-character limit

---

## 9. Storage Specification

## 9.1 Storage Model
Atlas shall use:
- SQLite as the canonical local store
- JSON sidecar artifacts for debug/inspection/export convenience
- content-hash-based invalidation
- repository-local `.atlas/` storage by default

## 9.2 On-Disk Layout

```text
.atlas/
  atlas.db
  manifest.json
  config.yaml
  summaries/
    files/
    packages/
    symbols/
  exports/
  runs/
  logs/
```

### 9.2.1 `atlas.db`
Primary SQLite store.

### 9.2.2 `manifest.json`
Contains repository identity, schema version, and generator metadata.

### 9.2.3 `config.yaml`
Repository-specific Atlas configuration.

### 9.2.4 `summaries/`
Optional sidecar summary JSON files for inspection/debugging.

### 9.2.5 `exports/`
Optional persisted export artifacts.

### 9.2.6 `runs/`
Optional run result snapshots and summaries.

### 9.2.7 `logs/`
Optional log output.

---

## 10. Database Schema

## 10.1 Schema Design Principles
1. Source files are the source of truth.
2. Database rows are machine memory.
3. Summaries are derived cache.
4. Hashes determine freshness.
5. Derived stale data must be detectable and rejectable.

## 10.2 Cross-Table Invariants
The following invariants cannot be expressed as DDL constraints and must be enforced at the application level. `atlas validate` must check all of them:
- `files.package_name` must equal `packages.name` for the file's package (via `package_files`) when non-NULL
- `files.module_name` must equal `packages.import_path` for the file's package (via `package_files`) when non-NULL
- symbols must only use `symbol_kind` values that the file's language extractor actually emits
- `artifacts.data_json` must contain all required keys for its `artifact_kind` (per Section 11.5); missing values must be stored as empty strings, not omitted

## 10.3 Required Tables
Atlas v1 must include at minimum:
- `schema_meta`
- `files`
- `packages`
- `package_files`
- `symbols`
- `references`
- `file_summaries`
- `package_summaries`
- `symbol_summaries`
- `artifacts`
- `index_runs`
- `diagnostics`

## 10.3 Timestamp Format
All `TEXT` fields storing timestamps (`created_at`, `updated_at`, `last_modified_utc`, `started_at`, `finished_at`) must use RFC 3339 format (e.g. `2024-01-15T09:30:00Z`). All timestamps must be in UTC.

## 10.4 Required DDL

```sql
CREATE TABLE schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE files (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    language TEXT NOT NULL,
    package_name TEXT,      -- denormalized from packages.name; NULL for files not belonging to a recognized package
    module_name TEXT,       -- denormalized from packages.import_path; NULL when language has no module concept
    content_hash TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    last_modified_utc TEXT,
    git_commit TEXT,          -- HEAD commit SHA at the time the file was last indexed; NULL if not a Git repo
    is_generated INTEGER NOT NULL DEFAULT 0,  -- 1 if file matches a generated-file heuristic: path contains 'generated', 'gen/', or file begins with '// Code generated'
    parse_status TEXT NOT NULL CHECK (parse_status IN ('ok', 'error', 'partial', 'skipped')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- A package is uniquely identified by its directory_path. import_path
-- is optional (e.g. JS packages may lack one) but when present must
-- be globally unique.
CREATE TABLE packages (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    import_path TEXT UNIQUE,
    directory_path TEXT NOT NULL UNIQUE,
    language TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE package_files (
    package_id INTEGER NOT NULL,
    file_id INTEGER NOT NULL,
    PRIMARY KEY (package_id, file_id),
    FOREIGN KEY(package_id) REFERENCES packages(id),
    FOREIGN KEY(file_id) REFERENCES files(id)
);

CREATE TABLE symbols (
    id INTEGER PRIMARY KEY,
    file_id INTEGER NOT NULL,
    package_id INTEGER,
    name TEXT NOT NULL,
    qualified_name TEXT NOT NULL,
    symbol_kind TEXT NOT NULL CHECK (symbol_kind IN ('package', 'function', 'method', 'struct', 'interface', 'type', 'const', 'var', 'field', 'enum', 'test', 'benchmark', 'class', 'module', 'trait', 'protocol', 'entrypoint')),
    visibility TEXT NOT NULL CHECK (visibility IN ('exported', 'unexported')),
    parent_symbol_id INTEGER,
    signature TEXT,
    doc_comment TEXT,
    start_line INTEGER,
    end_line INTEGER,
    stable_id TEXT NOT NULL UNIQUE,  -- format: '<language>:<qualified_name>:<symbol_kind>' (e.g. 'go:http.ListenAndServe:function'); stable across runs for unchanged symbols
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files(id),
    FOREIGN KEY(package_id) REFERENCES packages(id),
    FOREIGN KEY(parent_symbol_id) REFERENCES symbols(id)
);

-- References always have a source file. from_symbol_id and to_symbol_id
-- may be NULL for file-level references (e.g. imports) where no specific
-- symbol is the source or target. to_file_id may be NULL for unresolved
-- external references.
CREATE TABLE references (
    id INTEGER PRIMARY KEY,
    from_symbol_id INTEGER,
    to_symbol_id INTEGER,
    from_file_id INTEGER NOT NULL,
    to_file_id INTEGER,
    reference_kind TEXT NOT NULL CHECK (reference_kind IN ('calls', 'implements', 'imports', 'embeds', 'extends', 'instantiates', 'reads', 'writes', 'registers_route', 'uses_config', 'touches_table', 'tests', 'migrates', 'invokes_external_api')),
    confidence TEXT NOT NULL CHECK (confidence IN ('exact', 'likely', 'heuristic')),
    line INTEGER,
    column_start INTEGER,
    column_end INTEGER,
    raw_target_text TEXT,
    is_resolved INTEGER NOT NULL DEFAULT 1,  -- 1 when to_symbol_id is populated; 0 when the target symbol could not be resolved (e.g. external dependency, deleted symbol)
    created_at TEXT NOT NULL,
    FOREIGN KEY(from_symbol_id) REFERENCES symbols(id) ON DELETE CASCADE,
    FOREIGN KEY(to_symbol_id) REFERENCES symbols(id) ON DELETE SET NULL,
    FOREIGN KEY(from_file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY(to_file_id) REFERENCES files(id) ON DELETE SET NULL
);

CREATE TABLE file_summaries (
    file_id INTEGER PRIMARY KEY,
    summary_text TEXT NOT NULL,
    responsibilities_json TEXT,
    key_symbols_json TEXT,
    invariants_json TEXT,
    side_effects_json TEXT,
    dependencies_json TEXT,
    public_api_json TEXT,
    risks_json TEXT,
    related_artifacts_json TEXT,
    generated_from_hash TEXT NOT NULL,
    generator_version TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE TABLE package_summaries (
    package_id INTEGER PRIMARY KEY,
    summary_text TEXT NOT NULL,
    major_responsibilities_json TEXT,
    exported_surface_json TEXT,
    internal_collaborators_json TEXT,
    external_dependencies_json TEXT,
    notable_invariants_json TEXT,
    risks_json TEXT,
    generated_from_hash TEXT NOT NULL,
    generator_version TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(package_id) REFERENCES packages(id) ON DELETE CASCADE
);

CREATE TABLE symbol_summaries (
    symbol_id INTEGER PRIMARY KEY,
    summary_text TEXT NOT NULL,
    intent_json TEXT,
    inputs_json TEXT,
    outputs_json TEXT,
    side_effects_json TEXT,
    failure_modes_json TEXT,
    invariants_json TEXT,
    related_symbols_json TEXT,
    generated_from_hash TEXT NOT NULL,
    generator_version TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE
);

-- Every artifact must have a source file. symbol_id is optional
-- because some artifacts (e.g. env_var, migration) are file-level
-- rather than symbol-level.
CREATE TABLE artifacts (
    id INTEGER PRIMARY KEY,
    artifact_kind TEXT NOT NULL CHECK (artifact_kind IN ('route', 'config_key', 'migration', 'sql_query', 'background_job', 'queue_consumer', 'external_service', 'cli_command', 'env_var', 'feature_flag')),
    name TEXT NOT NULL,
    file_id INTEGER NOT NULL,
    symbol_id INTEGER,
    data_json TEXT NOT NULL,
    confidence TEXT NOT NULL CHECK (confidence IN ('exact', 'likely', 'heuristic')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE SET NULL
);

CREATE TABLE index_runs (
    id INTEGER PRIMARY KEY,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('running', 'success', 'partial', 'failed')),
    mode TEXT NOT NULL CHECK (mode IN ('full', 'incremental')),
    files_scanned INTEGER NOT NULL DEFAULT 0,
    files_changed INTEGER NOT NULL DEFAULT 0,
    files_reparsed INTEGER NOT NULL DEFAULT 0,
    symbols_written INTEGER NOT NULL DEFAULT 0,
    references_written INTEGER NOT NULL DEFAULT 0,
    summaries_written INTEGER NOT NULL DEFAULT 0,
    artifacts_written INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    warning_count INTEGER NOT NULL DEFAULT 0,
    git_commit TEXT,          -- HEAD commit SHA when the run started; NULL if not a Git repo
    notes TEXT
);

CREATE TABLE diagnostics (
    id INTEGER PRIMARY KEY,
    run_id INTEGER NOT NULL,
    file_id INTEGER,
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error', 'fatal')),
    code TEXT NOT NULL,
    message TEXT NOT NULL,
    line INTEGER,
    column_start INTEGER,
    column_end INTEGER,
    details_json TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY(run_id) REFERENCES index_runs(id),
    FOREIGN KEY(file_id) REFERENCES files(id)
);
```

## 10.4 Required Indexes

```sql
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_hash ON files(content_hash);
CREATE INDEX idx_packages_name ON packages(name);
CREATE INDEX idx_symbols_name ON symbols(name);
CREATE INDEX idx_symbols_qualified_name ON symbols(qualified_name);
CREATE INDEX idx_symbols_file_id ON symbols(file_id);
CREATE INDEX idx_references_from_symbol ON references(from_symbol_id);
CREATE INDEX idx_references_to_symbol ON references(to_symbol_id);
CREATE INDEX idx_references_kind ON references(reference_kind);
CREATE INDEX idx_artifacts_kind_name ON artifacts(artifact_kind, name);
CREATE INDEX idx_diagnostics_run_id ON diagnostics(run_id);
```

---

## 11. Domain Model

## 11.1 File
Represents one source file or related repository file.

### Required Fields
- path
- language
- content_hash
- size_bytes
- parse_status
- is_generated

### Optional Fields
- package_name (NULL for files not belonging to a recognized package)
- module_name (NULL when language has no module concept)

## 11.2 Package
Represents a logical package or module grouping.

### Required Fields
- name
- directory_path
- language

### Optional Fields
- import_path (NULL when language has no import path concept; globally unique when present)

## 11.3 Symbol
Represents a declaration or named code entity.

### Supported Kinds
- package
- function
- method
- struct
- interface
- type
- const
- var
- field
- enum
- test
- benchmark
- class
- module
- trait
- protocol
- entrypoint

### Qualified Name Format
The `qualified_name` field uses language-specific formats:
- **Go**: `package.SymbolName` for top-level symbols, `package.Type.Method` for methods (e.g. `http.ListenAndServe`, `Server.Serve`)
- **TypeScript/JavaScript**: `module.exportName` where `module` is the file path relative to repo root without extension (e.g. `src/utils/auth.validateToken`)

Each extractor must declare its supported symbol kinds via a `SupportedKinds() []string` method. Extractors must only emit symbol kinds present in their declared list. Queries must filter results to kinds supported by the relevant language extractor.

## 11.4 Reference
Represents a directional relationship.

### Supported Kinds
- calls
- implements
- imports
- embeds
- extends
- instantiates
- reads
- writes
- registers_route
- uses_config
- touches_table
- tests
- migrates
- invokes_external_api

### Confidence Values
- exact
- likely
- heuristic

## 11.5 Artifact
Represents extracted higher-level facts beyond basic symbol indexing.

### Supported Kinds and `data_json` Schema
Each artifact kind stores its data in the `data_json` field with the following required keys:

| Kind | Required `data_json` keys |
|------|--------------------------|
| `route` | `method`, `path`, `handler` |
| `config_key` | `key`, `default_value`, `source` |
| `migration` | `version`, `direction`, `description` |
| `sql_query` | `query_type`, `tables`, `raw_sql` |
| `background_job` | `job_name`, `schedule` |
| `queue_consumer` | `queue_name`, `handler` |
| `external_service` | `service_name`, `endpoint` |
| `cli_command` | `command_name`, `description` |
| `env_var` | `var_name`, `default_value` |
| `feature_flag` | `flag_name`, `default_value` |

All values are strings. Missing values must be stored as empty strings, not omitted.

---

## 12. Freshness and Invalidation Rules

## 12.1 Source of Truth Rule
Source code is the source of truth. All summaries and derived relationships are subordinate to source files.

## 12.2 Hash Rule
Every file must have a `content_hash`. Every summary must have a `generated_from_hash`. A summary is stale if its `generated_from_hash` does not equal the current file or package hash basis. Atlas must use SHA-256 for content hashing. The hash must produce identical output for identical input across all runs.

## 12.3 File Change Handling
When a file changes:
1. update file metadata and content hash
2. delete file-owned symbols (this cascades to delete their symbol summaries via ON DELETE CASCADE)
3. delete file-owned references (outgoing from this file)
4. set `to_symbol_id` to NULL on references from other files that pointed to deleted symbols (via ON DELETE SET NULL), marking them as `is_resolved = 0`
5. delete file summaries for the changed file
6. delete package summaries for packages containing the changed file
7. rebuild extracted facts from current file contents
8. re-resolve references from other files whose `to_symbol_id` was set to NULL in step 4: match by `raw_target_text` against the `qualified_name` of newly created symbols from step 7; if exactly one match is found, set `to_symbol_id` and `is_resolved = 1`; if zero or multiple matches are found, leave unresolved

## 12.4 File Deletion Handling
When a file is removed:
1. remove package membership for that file
2. remove file-owned symbols
3. remove file-owned references
4. remove file summaries
5. delete package summaries for packages that contained the deleted file
6. record deletion in run metadata

## 12.5 Rename Handling
v1 handles renames as a delete of the old path followed by an add of the new path. No VCS-based rename tracking is required.

## 12.6 Integrity Rule
If a file that exists in the `files` table is no longer found on disk during indexing, it is treated as deleted per Section 12.4. If a file exists on disk but is unreadable (e.g. permission denied), Atlas must record an `error` diagnostic and mark all derived records for that file as stale without deleting them. If a summary's source file hash cannot be verified, the summary must be treated as stale.

---

## 13. Language Support

## 13.1 Priority Order
Atlas v1 must support the following languages, listed in order of required feature completeness (most complete first):
1. Go
2. TypeScript
3. JavaScript

## 13.2 Shared Extractor Interface
Atlas must define a language extractor interface.

### Required Go Interface
```go
type Extractor interface {
	Language() string
	Supports(path string) bool
	SupportedKinds() []string
	Extract(ctx context.Context, req ExtractRequest) (*ExtractResult, error)
}

type ExtractRequest struct {
	RepoRoot string
	Path     string
	Content  []byte
}

type ExtractResult struct {
	File        FileRecord
	Package     *PackageRecord
	Symbols     []SymbolRecord
	References  []ReferenceRecord
	Artifacts   []ArtifactRecord
	Diagnostics []DiagnosticRecord
}
```

### Requirements
- language-specific parsing must be encapsulated behind this interface
- unsupported files must not crash indexing
- extractors must return whatever valid symbols and references were found before the error, along with a diagnostic of severity `error` describing the failure

---

## 14. Go Extractor Requirements

For Go, Atlas must extract at minimum:
- module information where available
- package declarations
- files
- functions
- methods and receiver information
- interfaces
- structs
- type aliases
- constants
- variables
- imports
- test functions
- benchmark functions
- interface implementations: detect types whose method sets match a declared interface in the same package or imported packages
- entrypoints: `main()` functions under `cmd/` directories
- route registrations: calls to `http.HandleFunc`, `http.Handle`, `mux.HandleFunc`, `router.GET/POST/PUT/DELETE` (net/http and gorilla/mux patterns)
- config/environment access: calls to `os.Getenv`, `os.LookupEnv`, `viper.Get*` patterns
- SQL/migration artifacts: string literals containing `CREATE TABLE`, `ALTER TABLE`, `INSERT`, `SELECT`, or files in directories named `migrations/` or `migrate/`

### Go Parsing Guidance
Atlas must use `go/parser` and `go/ast` from the Go standard library for all Go extraction. Regex-based extraction is not permitted for Go.

---

## 15. TypeScript/JavaScript Extractor Requirements

For TS/JS, Atlas must extract:
- modules
- imports/exports
- functions
- classes
- interfaces/types for TypeScript
- methods
- test files: files matching `*.test.ts`, `*.test.js`, `*.spec.ts`, `*.spec.js`; test cases: `describe()`, `it()`, `test()` calls
- v1 TS/JS support omits route handler and environment/config detection (see Section 15 omissions)

v1 TS/JS support omits: interface implementation detection, route registration detection, and config/environment access detection. These reference kinds must not be emitted for TS/JS files. The `atlas stats` output must report which extractor capabilities are active per language.

---

## 16. Semantic Summary Requirements

## 16.1 Summary Philosophy
Summaries must be structured JSON objects with short-phrase values. The `summary_text` field must not exceed 500 characters. Array fields must contain concise phrases (under 100 characters each), not sentences or paragraphs.

## 16.2 File Summary Fields
Each file summary must include:
- purpose
- responsibilities
- key symbols
- dependencies
- side effects
- invariants
- risks
- public_api
- related_artifacts

## 16.3 Package Summary Fields
Each package summary must include:
- purpose
- major_responsibilities
- exported_surface
- internal_collaborators
- external_dependencies
- notable_invariants
- risks

## 16.4 Symbol Summary Fields
Each symbol summary must include:
- intent
- inputs
- outputs
- side_effects
- failure_modes
- invariants
- related_symbols

## 16.5 Summary Generation Controls
Atlas configuration must allow:
- all summaries disabled
- only file summaries
- file plus package summaries
- selective symbol summaries
- selective regeneration

---

## 17. Query Semantics

## 17.1 Resolution Order
For symbol queries, Atlas should attempt resolution in this order:
1. exact qualified name
2. exact stable ID
3. exact simple name
4. fuzzy match if enabled

## 17.2 Ranking Rules
Default result ranking must apply these criteria in strict precedence order (highest first):
1. **Exactness**: exact qualified name > exact simple name > substring match
2. **Visibility**: exported/public symbols rank above unexported/private
3. **Symbol kind**: functions and types rank above fields, variables, and constants
4. **Relationship directness**: direct relationships rank above transitive
5. **File modification recency**: more recently modified files rank higher among otherwise equal results

## 17.3 Relationship Reporting
Relationship results must indicate:
- source entity
- target entity
- relationship kind
- confidence
- source path and location
- freshness status if affected by stale records

---

## 18. Configuration Specification

## 18.1 File Location
Default configuration path:

```text
.atlas/config.yaml
```

## 18.2 Required Config Fields
- version
- repo_root
- storage_dir
- include globs (Go `doublestar` syntax, e.g. `**/*.go`)
- exclude globs (Go `doublestar` syntax, e.g. `vendor/**`)
- language enablement
- summary settings: `enabled` (bool), `file` (bool), `package` (bool), `symbol` (bool)
- indexing settings: `ignore_generated_files` (bool), `max_file_size_bytes` (int), `use_git` (bool), `strict_mode` (bool)
- output settings: `default_format` (one of `text`, `json`, `agent`), `show_freshness` (bool)

## 18.3 Example Config

```yaml
version: 1

repo_root: .
storage_dir: .atlas

include:
  - "**/*.go"
  - "**/*.ts"
  - "**/*.tsx"
  - "**/*.js"
  - "**/*.jsx"

exclude:
  - ".git/**"
  - "node_modules/**"
  - "vendor/**"
  - "dist/**"
  - "build/**"
  - "coverage/**"
  - "**/*.min.js"

languages:
  go:
    enabled: true
  typescript:
    enabled: true
  javascript:
    enabled: true

summaries:
  enabled: true
  file: true
  package: true
  symbol: false

indexing:
  ignore_generated_files: true
  max_file_size_bytes: 2097152
  use_git: true
  strict_mode: false

output:
  default_format: text
  show_freshness: true
```

---

## 19. Service Capabilities (Non-Normative Design Guidance)

The following describes expected internal capabilities for implementation planning. These are not external requirements:

### 19.1 Repository Discovery
Locate the repository root given a starting path and configuration.

### 19.2 File Walking
Discover candidate files under the repository root, filtered by include/exclude globs and max file size.

### 19.3 Content Hashing
Compute deterministic content hashes for file change detection. The algorithm must be stable across runs for the same file content.

### 19.4 Index Orchestration
Coordinate init, incremental index, and full reindex operations. Responsible for file scanning, extraction dispatch, invalidation, persistence, and run metadata.

### 19.5 Query Resolution
Resolve symbol, file, package, and relationship queries against the persisted index. Must support the resolution order and ranking rules defined in Section 17.

### 19.6 Summary Generation
Generate and manage file, package, and symbol summaries with freshness tracking per Section 16.

### 19.7 Validation
Perform health checks (doctor) and consistency validation against the database and repository state per Section 23.

---

## 20. Indexing Flow (Non-Normative Design Guidance)

## 21.1 `atlas init` Flow
1. resolve repository root
2. create `.atlas/` directory if needed
3. create/open SQLite DB
4. run migrations
5. write manifest and config if missing
6. return repository initialization summary

## 21.2 `atlas index` Flow
1. resolve repository root and config
2. start index run row
3. discover candidate files
4. compute content hashes for candidate files (may use file modification time as a fast pre-check to skip hash computation for files whose mtime has not changed, but content hashes remain the source of truth)
5. determine changed/unchanged/deleted files
6. remove deleted file-owned data
7. for each changed file:
   - parse via language extractor
   - persist file metadata
   - replace file symbols
   - replace file references
   - replace file artifacts
   - invalidate dependent summaries
   - record diagnostics
8. regenerate enabled summaries as needed
9. finalize run stats
10. return index result

## 21.3 `atlas reindex` Flow
1. start full rebuild run
2. clear derived data safely
3. rescan all candidate files
4. rebuild all supported facts
5. regenerate enabled summaries
6. finalize run stats

---

## 22. Diagnostics and Failure Handling

## 22.1 Principle
Atlas must tolerate malformed or unsupported files without collapsing the entire run.

## 22.2 Severity Levels
Diagnostics must support:
- info
- warning
- error
- fatal

## 22.3 Failure Policy
- parse failures for one file should not abort indexing by default
- extractor partial failure should preserve whatever valid data can be extracted
- summary generation failure should not remove structural data
- strict mode must fail the run on any diagnostic of severity `error` or `fatal`

## 22.4 Diagnostic Categories
Every diagnostic must use one of the following codes:
- `PARSE_ERROR`
- `EXTRACT_PARTIAL`
- `SUMMARY_FAILED`
- `CONFIG_INVALID`
- `SCHEMA_MISMATCH`
- `ORPHANED_REFERENCE`
- `FILE_MISSING`
- `UNSUPPORTED_LANGUAGE`

---

## 23. Validation and Doctor Requirements

## 23.1 `atlas doctor`
Must check exactly the following for v1:
- `.atlas/` directory exists and is writable
- DB opens successfully and responds to a test query
- schema version stored in `schema_meta` is compatible with the current Atlas binary (the binary must define a `schema_version` constant and reject databases with a newer schema version)
- manifest exists and its repository root matches the detected root
- error and warning counts from the most recent index run
- count of stale file, package, and symbol summaries
- count of files referenced in DB but missing on disk
- SQLite integrity check (`PRAGMA integrity_check`)

## 23.2 `atlas validate`
Must check exactly the following for v1:
- foreign key consistency (`PRAGMA foreign_key_check`)
- no duplicate `stable_id` values in the `symbols` table
- no symbols referencing a `file_id` that does not exist in `files`
- no `package_files` rows referencing nonexistent `package_id` or `file_id`
- stale summaries (where `generated_from_hash` differs from the current file `content_hash`) are correctly detectable by `atlas stale`; validate checks that no summary claims freshness while its source file hash has changed
- all files in the `files` table exist on disk
- denormalized fields `files.package_name` and `files.module_name` match the corresponding `packages.name` and `packages.import_path` for the file's package
- `summary_text` fields do not exceed 500 characters
- array fields in summary JSON (e.g. `responsibilities_json`, `invariants_json`) contain no entries exceeding 100 characters
- in `--strict` mode: all files matched by include globs exist in the `files` table

---

## 24. Export Specification

## 24.1 `atlas export summary`
Must produce a JSON object with these top-level keys:
- `repo_root` (string): absolute path
- `last_run` (object): `started_at`, `status`, `mode`, `files_scanned`, `files_changed`
- `languages` (array of strings): languages with indexed files
- `packages` (array of objects): `name`, `import_path`, `file_count`
- `entrypoints` (array of objects): `name`, `file`, `line`
- `route_count` (integer)
- `diagnostics` (object): `error_count`, `warning_count`
- `stale_counts` (object): `file_summaries`, `package_summaries`, `symbol_summaries`

## 24.2 `atlas export graph`
Must produce a JSON object with:
- `nodes`: array of objects with `id` (stable_id), `name`, `kind` (symbol_kind), `file`, `language`
- `edges`: array of objects with `from` (source stable_id), `to` (target stable_id or raw_target_text if unresolved), `kind` (reference_kind), `confidence`

## 24.3 `atlas export symbols`
Must produce a JSON array of objects, each with:
- `name` (string)
- `qualified_name` (string)
- `kind` (string): symbol_kind value
- `visibility` (string): `exported` or `unexported`
- `file` (string): relative path
- `start_line` (integer)
- `end_line` (integer)
- `stable_id` (string)

---

## 25. Performance Requirements

### 25.1 Query Performance
Single-symbol, single-file, and single-package queries must return within 200ms on repositories with up to 5,000 source files.

### 25.2 Incremental Performance
Incremental indexing of a change set affecting fewer than 5% of files must complete in under 20% of the time required for a full index of the same repository.

### 25.3 Scale Target
Atlas v1 must index repositories with up to 10,000 source files. Full indexing of a 5,000-file Go repository must complete within 60 seconds on commodity hardware (4-core, 16 GB RAM, SSD).

---

## 26. Security Requirements

### 26.1 Local-Only Default
Atlas must operate entirely locally by default.

### 26.2 No Code Execution
Atlas must parse files and extract facts without executing repository code.

### 26.3 Safe Path Handling
Atlas must canonicalize paths and avoid path traversal issues.

### 26.4 No Silent Exfiltration
Atlas must not send repository contents to remote services unless an explicit future optional feature is configured.

---

## 27. Testing Requirements

## 27.1 Unit Tests
Must cover:
- config loading
- repo detection
- path normalization
- hashing
- invalidation rules
- schema migrations
- query logic
- output formatting

## 27.2 Integration Tests
Must cover:
- init/index/query workflow
- incremental update workflow
- file deletion handling
- stale summary detection
- export generation
- doctor and validate behavior

## 27.3 Fixture Repositories
Atlas should include fixture repositories for:
- Go-only repo
- TS-only repo
- mixed repo
- malformed repo
- generated-files repo

## 27.4 Determinism Tests
Repeated indexing of the same fixture with the same Atlas version and config must produce identical extracted facts (symbols, references, artifacts) and summaries. Only timestamps and internal database row IDs may differ between runs.

---

## 28. Acceptance Criteria

Atlas v1 is acceptable when:
1. `atlas init` creates usable repository-local storage.
2. `atlas index` fully indexes a supported Go repository.
3. files, symbols, references, artifacts, and runs persist in SQLite.
4. incremental indexing updates changed files without full rebuild.
5. stale summaries are correctly detected after source edits.
6. symbol lookup queries work reliably.
7. relationship queries return useful and location-aware results.
8. JSON output is stable enough for agent consumption.
9. doctor and validate commands detect common integrity issues.
10. the tool runs locally without any network dependency.
11. the CLI is written in Go and packaged as a standalone binary.

---

## 29. Out of Scope for v1
- embeddings as a primary storage/query mechanism
- cross-repository graphing
- daemon or watch mode
- full LSP integration
- remote/shared index service
- auto-editing code
- distributed repository intelligence

---

## 31. Final Product Statement

Atlas is a repository intelligence CLI for AI coding workflows. It creates a durable, incrementally maintained map of repository structure, relationships, and compact semantic summaries so agents and developers can navigate a codebase with less repeated searching, less repeated rereading, and better targeting of changes.

In plain English: it is tags after they got smarter, more honest, and useful to machines instead of just cranky old Vim users.

