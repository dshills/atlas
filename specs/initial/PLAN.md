# PLAN.md — Atlas Implementation Plan

## Overview

This plan implements Atlas, a Go CLI that builds and maintains a structural and semantic index of source repositories in SQLite. The plan is organized into 10 phases, each producing a testable increment. Phases are ordered by dependency: foundation first, then structural indexing, then queries, then higher-level features.

**Total estimated phases:** 10
**Critical path:** Phase 1 → 2 → 3 → 4 → 5 (foundation through Go queries)

---

## Phase 1 — Project Skeleton and Storage Foundation

### Goal
Establish the Go module, CLI framework, configuration loading, SQLite database with full schema, and the `atlas init` command.

### Spec References
- Section 5 (Repository Model), Section 7.1.1 (init), Section 9 (Storage), Section 10 (Database Schema), Section 18 (Configuration)

### Deliverables

#### 1.1 Go Module and CLI Framework
- Initialize Go module (`github.com/dshills/atlas` or chosen path)
- Set up `cmd/atlas/main.go` as the CLI entrypoint
- Use `cobra` for command registration
- Register placeholder subcommands for all core commands from Section 7.1: `init`, `index`, `reindex`, `stale`, `stats`, `doctor`, `validate`, `version`, `config`
- Implement `atlas version` (Section 7.1.8): print binary version constant, schema version constant, and `runtime.Version()`
- Support `--json` flag on `version`

#### 1.2 Configuration
- Create `internal/config/` package
- Define `Config` struct matching Section 18.2: `version`, `repo_root`, `storage_dir`, include/exclude globs, language enablement, summary settings, indexing settings, output settings
- Implement config loading from `.atlas/config.yaml` with defaults
- Implement `atlas config` command (Section 7.1.9): print effective merged config
- Support `--json` flag on `config`
- Use `gopkg.in/yaml.v3` for YAML parsing
- Use `github.com/bmatcuk/doublestar/v4` for glob pattern support (Section 18.2 specifies doublestar syntax)

#### 1.3 Repository Root Detection
- Create `internal/repo/` package
- Implement `FindRoot(start string, flagRepo string, cfg *Config) (string, error)` following Section 5.2 precedence:
  1. Explicit `--repo` flag
  2. Configured `repo_root` in config
  3. Git root (walk up looking for `.git/`)
  4. Current working directory
- Canonicalize all paths (Section 26.3)

#### 1.4 SQLite Database and Schema Migrations
- Create `internal/db/` package
- Use `modernc.org/sqlite` (pure-Go SQLite, no CGO)
- Implement migration framework: embed SQL migrations, track applied versions in `schema_meta`
- Rollback strategy: delete the DB file and re-run `atlas init` to rebuild from scratch (SQLite's `ALTER TABLE` limitations make incremental rollback impractical; full rebuild is safe for a local index tool since the source of truth is the repository itself)
- Migration 001: full DDL from Section 10.4 — all 12 `CREATE TABLE` statements (with CHECK constraints and ON DELETE actions) plus all 11 `CREATE INDEX` statements
- Enable `PRAGMA foreign_keys = ON` on every connection
- Enable WAL mode for concurrent read performance
- Define `schema_version` constant in binary; reject databases with newer schema version (Section 23.1)

#### 1.5 `atlas init` Command
- Implement full behavior from Section 7.1.1:
  - Create `.atlas/` directory if missing
  - Create SQLite DB and run migrations; fail with non-zero exit code if migrations fail
  - Write default `config.yaml` if missing (Section 18.3 example)
  - Write `manifest.json` with repo root, schema version, generator version
  - Initialize `schema_meta` with `schema_version` and `generator_version`
  - Print human-readable summary
- Support `--repo` flag for explicit repo root
- Support `--json` for JSON output

#### 1.6 Output Formatting Foundation
- Create `internal/output/` package
- Define `Formatter` interface with methods for text, JSON, and agent output modes (Section 8)
- JSON mode: standard `encoding/json` with indentation
- Agent mode: `encoding/json` with no indentation, omit null/empty fields (Section 8.3)
- Text mode: basic tabwriter-based formatting, no ANSI codes in agent mode
- All commands receive a `--json` and `--agent` flag

### Tests
- Unit: config loading with defaults, config merge, YAML parsing edge cases
- Unit: repo root detection with mock filesystem
- Unit: migration framework, schema creation, version checking
- Integration: `atlas init` creates `.atlas/`, DB, config, manifest; re-running is idempotent
- Integration: `atlas version` and `atlas config` produce correct output

### Files Created
```
cmd/atlas/main.go
internal/config/config.go
internal/config/config_test.go
internal/config/defaults.go
internal/repo/root.go
internal/repo/root_test.go
internal/db/db.go
internal/db/db_test.go
internal/db/migrations/001_initial.sql
internal/db/migrate.go
internal/output/formatter.go
internal/output/text.go
internal/output/json.go
internal/output/agent.go
internal/model/manifest.go
go.mod
go.sum
```

### Exit Criteria
- `atlas init` creates a valid `.atlas/` directory with DB, config, and manifest
- `atlas version` and `atlas config` produce correct output in text and JSON
- Database contains all 12 tables with correct constraints
- `golangci-lint run ./...` passes

---

## Phase 2 — File Walking, Hashing, and Index Run Infrastructure

### Goal
Implement file discovery, SHA-256 content hashing, the index run lifecycle, and the diagnostics system. After this phase, `atlas index` can scan and hash files but not yet parse them.

### Spec References
- Section 7.1.2 (index behavior), Section 12.2 (hash rule), Section 19.2 (file walking), Section 19.3 (hashing), Section 22 (diagnostics)

### Deliverables

#### 2.1 File Walker
- Create `internal/fswalk/` package
- Implement `Walk(root string, include []string, exclude []string, maxFileSize int64) ([]FileCandidate, error)`
- `FileCandidate`: path, size, mtime, language (detected from extension)
- Apply include/exclude globs using `doublestar.Match`
- Skip files exceeding `max_file_size_bytes`
- Detect language from file extension: `.go` → `go`, `.ts`/`.tsx` → `typescript`, `.js`/`.jsx` → `javascript`
- Generated file detection: path contains `generated`, `gen/`, or first line starts with `// Code generated` (Section 10.4 DDL comment)

#### 2.2 Content Hashing
- Create `internal/hash/` package
- Implement `Hash(content []byte) string` using SHA-256 (Section 12.2)
- Return lowercase hex-encoded hash string

#### 2.3 Diagnostics System
- Create `internal/diag/` package
- Define `Diagnostic` struct: severity (`info`/`warning`/`error`/`fatal`), code (Section 22.4 enum), message, file_id, line, column_start, column_end, details
- Define `Collector` that accumulates diagnostics during a run
- Persist diagnostics to `diagnostics` table at end of run

#### 2.4 Index Run Lifecycle
- Create `internal/indexer/` package
- Implement run lifecycle:
  1. Insert `index_runs` row with `status = 'running'`, `mode = 'full'|'incremental'`, `started_at` (RFC 3339 UTC)
  2. Walk files
  3. Hash files
  4. Compare hashes against `files` table to determine changed/unchanged/deleted
  5. Record deleted files (Section 12.4)
  6. Update run row with stats and `status = 'success'|'partial'|'failed'`, `finished_at`
- In strict mode, fail the run on any `error` or `fatal` diagnostic (Section 22.3)
- Record `git_commit` from current HEAD if in a Git repo

#### 2.5 File Metadata Persistence
- Create `internal/store/` package (persistence logic on top of `internal/db/`)
- Implement CRUD for `files` table:
  - Upsert file row with path, language, content_hash, size_bytes, last_modified_utc, git_commit, is_generated, parse_status
  - Delete file row (cascades to symbols, references, summaries, artifacts)
  - Query file by path, by hash
- Implement CRUD for `packages` table:
  - Upsert by directory_path
  - Link files via `package_files`
- Set `files.package_name` and `files.module_name` denormalized fields, maintaining invariant from Section 10.2
- All timestamps in RFC 3339 UTC (Section 10.3)

#### 2.6 VCS Integration
- Create `internal/vcs/` package
- Implement `GitRoot(start string) (string, error)` — walk up to find `.git/`
- Implement `HeadCommit(repoRoot string) (string, error)` — get current HEAD SHA
- Implement `DiffFiles(repoRoot string, since string) ([]string, error)` — files changed since a revision via `git diff --name-only`
- If `--since` is used and repo is not Git or revision is invalid, return error (Section 7.1.2); CLI must exit non-zero with an error diagnostic

#### 2.7 Skeleton `atlas index` and `atlas reindex`
- Wire up `atlas index` to: walk, hash, diff, persist file metadata, record run
- Wire up `atlas reindex` to: delete all derived data (Section 7.1.3 behavior), then run full index
- No parsing yet — files get `parse_status = 'skipped'`
- Support `--json`, `--since`, `--strict` flags

### Tests
- Unit: file walker with include/exclude patterns, max size, language detection, generated file detection
- Unit: SHA-256 hashing produces stable output
- Unit: diagnostic collector
- Unit: file change detection (new/changed/deleted/unchanged)
- Integration: `atlas index` on fixture repo creates file rows, records run
- Integration: `atlas reindex` clears and rebuilds
- Integration: `--since` with Git fixture

### Files Created
```
internal/fswalk/walk.go
internal/fswalk/walk_test.go
internal/hash/hash.go
internal/hash/hash_test.go
internal/diag/diagnostic.go
internal/diag/collector.go
internal/diag/collector_test.go
internal/indexer/indexer.go
internal/indexer/run.go
internal/indexer/run_test.go
internal/store/files.go
internal/store/packages.go
internal/store/runs.go
internal/store/diagnostics.go
internal/vcs/git.go
internal/vcs/git_test.go
testdata/fixtures/go-only/          (Go fixture repo)
testdata/fixtures/malformed/        (malformed files fixture)
```

### Exit Criteria
- `atlas index` walks a repo, hashes files, persists file metadata with correct hashes
- Re-running `atlas index` on unchanged files reports 0 changes
- Modifying a file and re-running reports the file as changed
- Deleting a file and re-running removes it from DB
- `atlas reindex` clears and rebuilds all file metadata
- `golangci-lint run ./...` passes

---

## Phase 3 — Go Extractor and Structural Indexing

### Goal
Implement the Go language extractor using `go/parser` and `go/ast`. After this phase, `atlas index` parses Go files and populates symbols and packages.

### Spec References
- Section 13 (Language Support), Section 14 (Go Extractor), Section 11.3 (Symbol), Section 11.2 (Package)

### Deliverables

#### 3.1 Extractor Interface
- Create `internal/extractor/` package
- Define `Extractor` interface per Section 13.2:
  ```go
  type Extractor interface {
      Language() string
      Supports(path string) bool
      SupportedKinds() []string
      Extract(ctx context.Context, req ExtractRequest) (*ExtractResult, error)
  }
  ```
- Define `ExtractRequest`, `ExtractResult`, and record types: `FileRecord`, `PackageRecord`, `SymbolRecord`, `ReferenceRecord`, `ArtifactRecord`, `DiagnosticRecord`
- Create extractor registry: map language → extractor

#### 3.2 Go Extractor — Structural Facts
- Create `internal/extractor/goextractor/` package
- Use `go/parser.ParseFile` with `parser.ParseComments` to parse Go files
- Extract the following (Section 14):
  - **Package**: from `ast.File.Name`, derive `PackageRecord` with name and directory path
  - **Module**: parse `go.mod` if present to get module import path
  - **Functions**: `ast.FuncDecl` without receiver → kind `function`
  - **Methods**: `ast.FuncDecl` with receiver → kind `method`, include receiver type
  - **Structs**: `ast.TypeSpec` with `ast.StructType` → kind `struct`, extract fields as child symbols (kind `field`)
  - **Interfaces**: `ast.TypeSpec` with `ast.InterfaceType` → kind `interface`
  - **Type aliases**: `ast.TypeSpec` that is not struct/interface → kind `type`
  - **Constants**: `ast.GenDecl` with `token.CONST` → kind `const`
  - **Variables**: `ast.GenDecl` with `token.VAR` → kind `var`
  - **Imports**: `ast.ImportSpec` → `ReferenceRecord` with kind `imports`
  - **Tests**: functions named `Test*` with `*testing.T` parameter → kind `test`
  - **Benchmarks**: functions named `Benchmark*` with `*testing.B` → kind `benchmark`
  - **Entrypoints**: `main()` in files under `cmd/` directories → kind `entrypoint`
- For each symbol, generate:
  - `qualified_name`: `package.SymbolName` or `package.Type.Method` (Section 11.3)
  - `stable_id`: `go:<qualified_name>:<symbol_kind>` (Section 10.4 DDL comment)
  - `visibility`: `exported` if first letter is uppercase, `unexported` otherwise
  - `signature`: function/method signature string
  - `doc_comment`: from `ast.CommentGroup`
  - `start_line`, `end_line` from `token.FileSet`
- `SupportedKinds()` returns: `package`, `function`, `method`, `struct`, `interface`, `type`, `const`, `var`, `field`, `test`, `benchmark`, `entrypoint`
- On parse error: return partial results with `PARSE_ERROR` diagnostic (Section 13.2 requirements)

#### 3.3 Symbol and Package Persistence
- Extend `internal/store/`:
  - `UpsertSymbols(fileID int64, symbols []SymbolRecord)` — delete existing symbols for file, insert new ones
  - `UpsertPackage(pkg PackageRecord) (int64, error)` — upsert by directory_path
  - `LinkFileToPackage(fileID, packageID int64)`
  - Set `files.package_name` and `files.module_name` from package data

#### 3.4 Integrate Extractor into Indexer
- Update `internal/indexer/` to:
  1. For each changed file, select extractor by language
  2. Read file content
  3. Call `Extract()`
  4. Persist `FileRecord` metadata (update `parse_status` to `ok`, `error`, or `partial`)
  5. Persist `PackageRecord` and `package_files` link
  6. Persist `SymbolRecord` list
  7. Persist `ReferenceRecord` list (imports only in this phase)
  8. Record diagnostics
- Files with no matching extractor get `parse_status = 'skipped'` and an `UNSUPPORTED_LANGUAGE` diagnostic

### Tests
- Unit: Go extractor on fixture files covering functions, methods, structs, interfaces, types, consts, vars, imports, tests, benchmarks, entrypoints
- Unit: qualified_name and stable_id generation
- Unit: visibility detection
- Unit: parse error handling returns partial results
- Integration: `atlas index` on Go fixture repo populates symbols, packages, package_files
- Determinism: two runs on same fixture produce identical symbols (Section 27.4)

### Files Created
```
internal/extractor/extractor.go
internal/extractor/registry.go
internal/extractor/goextractor/go.go
internal/extractor/goextractor/go_test.go
internal/extractor/goextractor/symbols.go
internal/extractor/goextractor/imports.go
internal/extractor/goextractor/module.go
internal/store/symbols.go
internal/store/packages.go  (extend)
testdata/fixtures/go-only/cmd/server/main.go
testdata/fixtures/go-only/internal/handler/handler.go
testdata/fixtures/go-only/internal/handler/handler_test.go
testdata/fixtures/go-only/internal/model/model.go
testdata/fixtures/go-only/go.mod
```

### Exit Criteria
- `atlas index` on Go fixture repo creates correct symbol rows with proper kinds, names, qualified names, stable IDs, visibility, line ranges
- Packages and package_files links are correct
- Imports are recorded as references
- Parse errors produce diagnostics without aborting
- `golangci-lint run ./...` passes

---

## Phase 4 — Relationship Extraction and Artifacts

### Goal
Extend the Go extractor to detect calls, interface implementations, route registrations, config access, and SQL artifacts. Populate the `references` and `artifacts` tables.

### Spec References
- Section 11.4 (Reference), Section 11.5 (Artifact), Section 14 (Go Extractor — relationship items)

### Deliverables

#### 4.1 Call Detection
- Walk `ast.CallExpr` nodes to detect function/method calls
- Generate `ReferenceRecord` with kind `calls`, confidence `exact` for direct calls within the same package
- For cross-package calls: store `raw_target_text`, attempt to resolve `to_symbol_id` against indexed symbols; set confidence `likely` if resolved, `heuristic` if unresolved
- Set `is_resolved = 1` when `to_symbol_id` is populated, `0` otherwise

#### 4.2 Interface Implementation Detection
- For each struct type, collect its method set
- For each interface in the same package or imported packages, check if the struct's method set is a superset of the interface's method set
- Generate `ReferenceRecord` with kind `implements`, confidence `exact` for same-package, `likely` for cross-package

#### 4.3 Route Registration Detection
- Detect calls matching patterns from Section 14:
  - `http.HandleFunc(path, handler)`
  - `http.Handle(path, handler)`
  - `mux.HandleFunc(path, handler)` (gorilla/mux)
  - `router.GET/POST/PUT/DELETE(path, handler)`
- Generate `ArtifactRecord` with kind `route`, data_json: `{"method": "...", "path": "...", "handler": "..."}`
- Generate `ReferenceRecord` with kind `registers_route`
- Confidence: `exact` for literal string paths, `heuristic` for variable paths

#### 4.4 Config/Environment Access Detection
- Detect calls to `os.Getenv`, `os.LookupEnv`, `viper.Get*` patterns (Section 14)
- Generate `ArtifactRecord` with kind `env_var` or `config_key`
- Generate `ReferenceRecord` with kind `uses_config`

#### 4.5 SQL/Migration Artifact Detection
- Detect string literals containing `CREATE TABLE`, `ALTER TABLE`, `INSERT`, `SELECT`
- Detect files in `migrations/` or `migrate/` directories
- Generate `ArtifactRecord` with kind `sql_query` or `migration`
- Generate `ReferenceRecord` with kind `touches_table` or `migrates`

#### 4.6 Background Job and External Service Detection
- Detect goroutine launches (`go func()`, `go someName()`), common job patterns (`cron.AddFunc`, `scheduler.Every`) → artifact kind `background_job`
- Detect HTTP client calls (`http.Get`, `http.Post`, `http.NewRequest`, `rpc.Dial`, `grpc.Dial`) → artifact kind `external_service`
- Generate `ReferenceRecord` with kind `invokes_external_api` for external service artifacts
- Confidence: `heuristic` for all (pattern-matched, not guaranteed)

#### 4.7 Test Reference Detection
- For `Test*` and `Benchmark*` functions, attempt to identify the function/type under test by naming convention (e.g. `TestFoo` tests `Foo`)
- Generate `ReferenceRecord` with kind `tests`, confidence `heuristic`

#### 4.8 Artifact Persistence
- Extend `internal/store/`:
  - `UpsertArtifacts(fileID int64, artifacts []ArtifactRecord)`
  - `UpsertReferences(fileID int64, refs []ReferenceRecord)`
- Validate `data_json` contains all required keys per artifact kind (Section 11.5 table)
- Store empty strings for missing values, not omit them

#### 4.9 Cross-File Reference Resolution
- After all files in a run are indexed, attempt to resolve unresolved references:
  - Match `raw_target_text` against `symbols.qualified_name`
  - If exactly one match: set `to_symbol_id`, `to_file_id`, `is_resolved = 1`
  - If zero or multiple: leave `is_resolved = 0`

### Tests
- Unit: call detection on fixture functions with known call graphs
- Unit: interface implementation detection with test interfaces and structs
- Unit: route detection with net/http and gorilla/mux patterns
- Unit: env var and config detection
- Unit: SQL/migration artifact detection
- Unit: test reference detection by naming convention
- Integration: full index of Go fixture with routes, tests, config calls produces correct references and artifacts
- Integration: cross-file reference resolution

### Files Created
```
internal/extractor/goextractor/calls.go
internal/extractor/goextractor/implements.go
internal/extractor/goextractor/routes.go
internal/extractor/goextractor/config.go
internal/extractor/goextractor/sql.go
internal/extractor/goextractor/tests.go
internal/extractor/goextractor/calls_test.go
internal/extractor/goextractor/implements_test.go
internal/extractor/goextractor/routes_test.go
internal/store/references.go
internal/store/artifacts.go
testdata/fixtures/go-only/internal/handler/routes.go
testdata/fixtures/go-only/internal/config/config.go
testdata/fixtures/go-only/migrations/001_init.sql
```

### Exit Criteria
- References table populated with calls, implements, imports, tests, registers_route, uses_config, touches_table
- Artifacts table populated with routes, env_vars, config_keys, migrations
- Cross-file reference resolution works
- `data_json` contains all required keys for each artifact kind
- `golangci-lint run ./...` passes

---

## Phase 5 — Incremental Indexing and Invalidation

### Goal
Implement correct incremental indexing with the full invalidation cascade from Section 12.3, including cross-file reference re-resolution.

### Spec References
- Section 12 (Freshness and Invalidation), Section 7.1.2 (index), Section 7.1.3 (reindex)

### Deliverables

#### 5.1 Change Detection
- Compare content hashes of on-disk files against `files.content_hash`
- Optionally use file mtime as fast pre-check (Section 20, flow step 4)
- Categorize files as: new, changed, unchanged, deleted

#### 5.2 File Change Invalidation (Section 12.3)
Implement the full 8-step cascade:
1. Update file metadata and content hash
2. Delete file-owned symbols (cascades to symbol_summaries via ON DELETE CASCADE)
3. Delete file-owned references (outgoing)
4. ON DELETE SET NULL automatically nullifies `to_symbol_id` on references from other files; application must then set `is_resolved = 0` on those affected references
5. Delete file summaries
6. Delete package summaries for packages containing the changed file
7. Re-extract and persist new symbols, references, artifacts
8. Re-resolve: match affected references' `raw_target_text` against new `qualified_name` values; set `to_symbol_id` and `is_resolved = 1` on exact single matches

#### 5.3 File Deletion Handling (Section 12.4)
1. Remove `package_files` membership
2. Delete file row (cascades to symbols, references, summaries, artifacts)
3. Delete package summaries for affected packages
4. Record deletion in run notes

#### 5.4 Unreadable File Handling (Section 12.6)
- If file exists but is unreadable (e.g. permission denied), record `error` diagnostic with code `FILE_MISSING` (closest match in Section 22.4 allowed codes)
- Mark all derived records for that file as stale without deleting them

#### 5.5 Rename Handling (Section 12.5)
- Delete-then-add: no special logic

#### 5.6 Transactional Persistence
- Wrap the entire index run in a SQLite transaction
- On failure, rollback; update run status to `failed`
- On partial success (some files errored), commit successful files; update run status to `partial`

### Tests
- Integration: modify a file, re-index, verify only that file's symbols are rebuilt
- Integration: delete a file, re-index, verify file and its data are removed
- Integration: rename a file (delete + add), verify correct handling
- Integration: cross-file reference invalidation and re-resolution
- Integration: unreadable file produces diagnostic without crashing
- Integration: verify unchanged files are not re-parsed (check run stats)
- Performance: incremental index of <5% changes takes <20% of full index time (Section 25.2)

### Files Created
```
internal/indexer/incremental.go
internal/indexer/incremental_test.go
internal/indexer/invalidation.go
internal/indexer/invalidation_test.go
```

### Exit Criteria
- Modifying one file re-indexes only that file
- Cross-file references to deleted symbols become unresolved
- Cross-file references are re-resolved when symbols are recreated
- Package summaries are invalidated when member files change
- `golangci-lint run ./...` passes

---

## Phase 6 — Query Commands and Output

### Goal
Implement all query, relationship, and listing commands with text, JSON, and agent output modes.

### Spec References
- Section 7.2 (Query Commands), Section 7.3 (Relationship Commands), Section 7.5 (Listing Commands), Section 17 (Query Semantics), Section 8 (Output Modes)

### Deliverables

#### 6.1 Query Service
- Create `internal/query/` package
- Implement symbol resolution following Section 17.1 order:
  1. Exact `qualified_name` match
  2. Exact `stable_id` match
  3. Exact `name` match
  4. Case-insensitive substring match (when `--fuzzy`)
- Implement result ranking per Section 17.2 strict precedence:
  1. Exactness (qualified > simple > substring)
  2. Visibility (exported > unexported)
  3. Symbol kind (functions/types > fields/variables/constants)
  4. Relationship directness (direct > transitive)
  5. File modification recency

#### 6.2 `atlas find symbol`
- Accept `<name>` or `<qualified-name>` argument
- Support flags: `--kind`, `--package`, `--file`, `--language`, `--visibility`, `--fuzzy`, `--json`, `--agent`
- Output: file path, line range, kind, visibility, stable_id

#### 6.3 `atlas find file`
- Accept `<pattern>` argument
- Default: case-sensitive substring match on path
- `--exact`: exact path match
- `--include`/`--exclude`: glob filters on results
- Support `--json`, `--agent`

#### 6.4 `atlas find package`
- Accept `<name>` argument
- Match by `packages.name` or `packages.import_path`
- Support `--json`, `--agent`

#### 6.5 `atlas find route` and `atlas find config`
- Query `artifacts` table by kind and name pattern
- Return source location and confidence

#### 6.6 Relationship Commands
- `atlas who-calls <symbol>`: query references where `to_symbol_id` matches and `reference_kind = 'calls'`
- `atlas calls <symbol>`: query references where `from_symbol_id` matches and `reference_kind = 'calls'`
- `atlas implementations <interface>`: query references where `to_symbol_id` matches and `reference_kind = 'implements'`
- `atlas imports <package>`: query references where `reference_kind = 'imports'` and target package matches
- `atlas tests-for <target>`: query references where `reference_kind = 'tests'` and target matches
- `atlas touches <artifact-kind> <name>`: query artifacts by kind and name
- All relationship commands: resolve subject first, then query, rank by Section 17.2, include source path/location/confidence/freshness

#### 6.7 Listing Commands
- Implement all 7 list commands per Section 7.5 command-to-data mapping table:
  - `atlas list packages`, `atlas list routes`, `atlas list jobs`, `atlas list migrations`, `atlas list integrations`, `atlas list entrypoints`, `atlas list diagnostics`
- Support `--language`, `--name` (substring filter), `--sort` (`name` or `count`)
- Support `--json` output

#### 6.8 `atlas stats` Command
- Implement all required statistics from Section 7.1.5:
  - File counts by language, package count, symbol counts by kind, reference counts by kind, artifact counts by kind
  - Stale summary counts
  - Last run info
  - DB file size
  - Active extractor capabilities per language (from `SupportedKinds()`)

#### 6.9 `atlas stale` Command
- Query all summaries where `generated_from_hash` != current file `content_hash`
- Group by type (file/package/symbol)
- Support `--json`

### Tests
- Unit: symbol resolution order and ranking
- Unit: fuzzy matching
- Unit: relationship queries
- Integration: full init→index→query workflow on Go fixture
- Integration: each list command returns expected data
- Integration: stats output is accurate
- Performance: queries return within 200ms on 5,000-file fixture (Section 25.1)

### Files Created
```
internal/query/symbols.go
internal/query/files.go
internal/query/packages.go
internal/query/relationships.go
internal/query/artifacts.go
internal/query/ranking.go
internal/query/query_test.go
internal/cli/find.go
internal/cli/relationships.go
internal/cli/list.go
internal/cli/stats.go
internal/cli/stale.go
```

### Exit Criteria
- All `find`, relationship, and `list` commands work correctly
- Resolution order and ranking match spec
- Output modes (text, JSON, agent) work for all commands
- `atlas stats` reports accurate counts
- `golangci-lint run ./...` passes

---

## Phase 7 — Semantic Summaries

### Goal
Implement file, package, and symbol summary generation with freshness tracking and stale detection.

### Spec References
- Section 16 (Semantic Summary Requirements), Section 7.4 (Summary Commands)

### Deliverables

#### 7.1 Summary Generator
- Create `internal/summary/` package
- Define summary generation interface — summaries are structured data extracted from indexed symbols, not LLM-generated
- File summary: aggregate symbols, dependencies, public API from indexed data for the file
- Package summary: aggregate across all files in the package
- Symbol summary: extract from symbol metadata (signature, doc_comment, references)

#### 7.2 Summary Fields
- File summary (Section 16.2): purpose (from package doc or filename), responsibilities, key_symbols, dependencies, side_effects, invariants, risks, public_api, related_artifacts
- Package summary (Section 16.3): purpose, major_responsibilities, exported_surface, internal_collaborators, external_dependencies, notable_invariants, risks
- Symbol summary (Section 16.4): intent, inputs, outputs, side_effects, failure_modes, invariants, related_symbols

#### 7.3 Summary Constraints
- `summary_text` must not exceed 500 characters (Section 16.1)
- Array field entries must not exceed 100 characters each
- Store as JSON in the `*_json` columns

#### 7.4 Freshness Tracking
- Set `generated_from_hash` to the file's current `content_hash` when generating
- Set `generator_version` to current Atlas version
- A summary is stale when `generated_from_hash` != current `content_hash`

#### 7.5 Summary Generation Controls (Section 16.5)
- Respect config: `summaries.enabled`, `summaries.file`, `summaries.package`, `summaries.symbol`
- Support selective regeneration via `atlas summarize` commands

#### 7.6 Summary Commands
- `atlas summarize file <path>`: generate/retrieve file summary
- `atlas summarize package <name>`: generate/retrieve package summary
- `atlas summarize symbol <qualified-name>`: generate/retrieve symbol summary
- All support `--json` and `--agent`

#### 7.7 Summary Persistence
- Extend store with CRUD for `file_summaries`, `package_summaries`, `symbol_summaries`
- Auto-generate summaries during `atlas index` when enabled

### Tests
- Unit: summary generation produces correct fields within length limits
- Unit: freshness detection (stale vs fresh)
- Integration: index with summaries enabled populates summary tables
- Integration: modify file, re-index, verify summary is regenerated with new hash
- Integration: `atlas stale` detects stale summaries correctly

### Files Created
```
internal/summary/file.go
internal/summary/package.go
internal/summary/symbol.go
internal/summary/generator.go
internal/summary/generator_test.go
internal/store/summaries.go
internal/cli/summarize.go
```

### Exit Criteria
- Summaries generated for files and packages during indexing
- Summary fields respect length constraints
- Stale summaries detected after file changes
- `atlas summarize` commands work correctly
- `golangci-lint run ./...` passes

---

## Phase 8 — Doctor, Validate, and Integrity

### Goal
Implement the `atlas doctor` and `atlas validate` commands with all checks specified in Section 23.

### Spec References
- Section 23 (Validation and Doctor Requirements), Section 7.1.6 (doctor), Section 7.1.7 (validate)

### Deliverables

#### 8.1 `atlas doctor` (Section 23.1)
- Create `internal/doctor/` package
- Implement exactly these checks:
  1. `.atlas/` directory exists and is writable
  2. DB opens and responds to a test query (`SELECT 1`)
  3. `schema_meta` schema version compatible with binary (reject newer)
  4. `manifest.json` exists and repo root matches detected root
  5. Error and warning counts from most recent `index_runs` row
  6. Count of stale summaries (file, package, symbol)
  7. Count of files in DB but missing on disk
  8. SQLite integrity check (`PRAGMA integrity_check`)
- Report each check as pass/fail with details
- Support `--json`

#### 8.2 `atlas validate` (Section 23.2)
- Create `internal/validate/` package
- Implement exactly these checks:
  1. `PRAGMA foreign_key_check` — report violations
  2. No duplicate `stable_id` in `symbols`
  3. No symbols with `file_id` not in `files`
  4. No `package_files` with nonexistent `package_id` or `file_id`
  5. No summary claiming freshness while source hash has changed
  6. All files in `files` table exist on disk
  7. Denormalized `files.package_name` matches `packages.name`
  8. Denormalized `files.module_name` matches `packages.import_path`
  9. `summary_text` fields ≤ 500 characters
  10. Array fields in summary JSON: no entries > 100 characters
  11. `artifacts.data_json` contains all required keys per artifact kind
  12. `--strict`: all files matched by include globs exist in `files` table
- Report each check with pass/fail/count
- Exit 0 on all pass, non-zero on any failure
- Support `--json`

### Tests
- Integration: doctor on healthy repo passes all checks
- Integration: doctor on repo with missing DB reports failure
- Integration: validate on clean index passes
- Integration: validate detects orphaned references, stale summaries, missing files
- Integration: validate --strict detects unindexed files

### Files Created
```
internal/doctor/doctor.go
internal/doctor/doctor_test.go
internal/validate/validate.go
internal/validate/validate_test.go
internal/cli/doctor.go
internal/cli/validate.go
```

### Exit Criteria
- `atlas doctor` reports all 8 checks accurately
- `atlas validate` reports all checks from Section 23.2
- `--strict` mode catches unindexed files
- `golangci-lint run ./...` passes

---

## Phase 9 — Export Commands

### Goal
Implement all export commands with stable JSON schemas.

### Spec References
- Section 7.6 (Export Commands), Section 24 (Export Specification)

### Deliverables

#### 9.1 `atlas export summary` (Section 24.1)
- Produce JSON with: `repo_root`, `last_run` (object), `languages`, `packages` (array with file_count), `entrypoints`, `route_count`, `diagnostics` (error/warning counts), `stale_counts`

#### 9.2 `atlas export graph` (Section 24.2)
- Produce JSON with `nodes` (stable_id, name, kind, file, language) and `edges` (from, to, kind, confidence)

#### 9.3 `atlas export symbols` (Section 24.3)
- Produce JSON array with name, qualified_name, kind, visibility, file, start_line, end_line, stable_id

#### 9.4 `atlas export packages`
- Produce JSON array of packages with name, import_path, directory_path, language, file_count

#### 9.5 `atlas export routes`
- Produce JSON array of route artifacts with method, path, handler, file, line, confidence

#### 9.6 `atlas export diagnostics`
- Produce JSON array of diagnostics with severity, code, message, file, line

#### 9.7 Common Export Features
- All export commands support `--out <file>` to write to file instead of stdout
- Default to stdout
- Stable schema (fields always present, even if empty arrays)

### Tests
- Integration: each export command produces valid JSON matching its schema
- Integration: `--out` writes to file correctly
- Integration: export on empty index produces valid empty-state JSON

### Files Created
```
internal/export/summary.go
internal/export/graph.go
internal/export/symbols.go
internal/export/packages.go
internal/export/routes.go
internal/export/diagnostics.go
internal/export/export_test.go
internal/cli/export.go
```

### Exit Criteria
- All 6 export commands produce correct JSON matching Section 24 schemas
- `--out` flag works
- `golangci-lint run ./...` passes

---

## Phase 10 — TypeScript/JavaScript Extractor

### Goal
Implement the TS/JS language extractor with the v1 feature set defined in Section 15.

### Spec References
- Section 15 (TypeScript/JavaScript Extractor Requirements), Section 13.2 (Extractor Interface)

### Deliverables

#### 10.1 TS/JS Parser Selection
- Use a regex/heuristic-based extractor for v1 TS/JS; Section 15 defines a reduced scope that makes this viable
- Known limitation: regex-based extraction is brittle for edge cases (nested templates, computed names); this is accepted as tech debt for v1
- If accuracy proves insufficient during Phase 10 testing, escalate to a tree-sitter Go binding as a stretch goal

#### 10.2 TS/JS Extractor
- Create `internal/extractor/tsextractor/` package (handles both TS and JS)
- `Language()` returns `typescript` or `javascript` based on file extension
- `Supports()` matches `.ts`, `.tsx`, `.js`, `.jsx`
- `SupportedKinds()` returns: `module`, `function`, `method`, `class`, `interface`, `type`, `const`, `var`, `test`, `entrypoint`
- Extract:
  - Modules: from file path (module name = relative path without extension)
  - All symbols use `qualified_name` format `module.exportName` per Section 11.3 (e.g., `src/utils/auth.validateToken`)
  - Imports/exports: `import` and `export` statements → reference kind `imports`
  - Functions: `function` declarations and arrow function assignments
  - Classes: `class` declarations with methods as children
  - Interfaces/types: TypeScript `interface` and `type` declarations
  - Methods: class methods
  - Test detection: files matching `*.test.ts`, `*.test.js`, `*.spec.ts`, `*.spec.js`; `describe()`, `it()`, `test()` calls → kind `test`

#### 10.3 v1 Omissions
- No interface implementation detection
- No route registration detection
- No config/environment access detection
- These reference kinds must not be emitted for TS/JS files (Section 15)

#### 10.4 Register Extractor
- Add TS/JS extractor to extractor registry
- Update `atlas stats` to report TS/JS capabilities

#### 10.5 Mixed Repository Support
- Test indexing repos with both Go and TS/JS files
- Verify cross-language queries work (e.g. `atlas find symbol` searches both languages)

### Tests
- Unit: TS/JS extractor on fixture files with functions, classes, interfaces, imports, exports, tests
- Unit: correct qualified name generation for TS/JS
- Integration: index mixed Go+TS repo, verify both languages indexed
- Integration: verify omitted capabilities don't emit forbidden reference kinds
- Integration: `atlas stats` reports correct capabilities per language

### Files Created
```
internal/extractor/tsextractor/ts.go
internal/extractor/tsextractor/ts_test.go
internal/extractor/tsextractor/parser.go
testdata/fixtures/ts-only/src/index.ts
testdata/fixtures/ts-only/src/utils.ts
testdata/fixtures/ts-only/src/utils.test.ts
testdata/fixtures/mixed/                    (Go + TS files)
```

### Exit Criteria
- TS/JS files are indexed with correct symbols, packages, and import references
- Omitted capabilities (implements, routes, config) are not emitted
- Mixed-language repos index correctly
- `atlas stats` reports capabilities per language
- `golangci-lint run ./...` passes

---

## Phase Dependencies

```
Phase 1 (Skeleton/Storage)
  └─→ Phase 2 (File Walking/Hashing)
        └─→ Phase 3 (Go Extractor/Structural)
              ├─→ Phase 4 (Relationships/Artifacts)
              │     └─→ Phase 5 (Incremental Indexing)
              │           └─→ Phase 6 (Queries/Output)
              │                 ├─→ Phase 7 (Summaries)
              │                 │     └─→ Phase 8 (Doctor/Validate)
              │                 │           └─→ Phase 9 (Exports)
              │                 └─→ Phase 10 (TS/JS Extractor)
```

Phases 9 and 10 are independent of each other and can be developed in parallel after their respective dependencies are met.

---

## Cross-Cutting Concerns

### Testing Strategy
- **Fixture repos** in `testdata/fixtures/`: go-only, ts-only, mixed, malformed, generated-files (Section 27.3)
- **Unit tests** for all packages, run via `go test ./...`
- **Integration tests** tagged `//go:build integration` for full CLI workflows
- **Determinism tests** per Section 27.4: two runs produce identical facts

### Error Handling
- All errors return structured diagnostics, never panic
- Partial extraction always preferred over total failure
- Strict mode fails the run on `error`/`fatal` diagnostics

### Performance
- WAL mode for SQLite concurrent reads
- Batch inserts within transactions
- Mtime pre-check to skip unchanged files
- Target: <200ms queries, <60s full index on 5K Go files (Section 25)

### Security
- Canonicalize all paths (Section 26.3)
- No code execution during indexing (Section 26.2)
- No network access (Section 26.1)
