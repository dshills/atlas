# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Atlas is a Go CLI that builds and maintains a machine-optimized structural and semantic index of source repositories for AI coding agents. It uses SQLite for local storage, extracts symbols/relationships/artifacts from source code via language-specific extractors, and provides query commands for navigating codebases without rereading files.

## Build & Development Commands

```bash
# Build
go build ./cmd/atlas

# Run tests
go test ./...

# Run a single test
go test ./internal/indexer -run TestIncremental

# Lint (required after any Go changes)
golangci-lint run ./...
```

## Spec-Driven Workflow

Specs and plans live in `./specs/`. Follow this sequence:

1. `/spec-review` — Validate SPEC.md with speccritic
2. `/plan` — Generate PLAN.md from spec, validate with plancritic
3. `/implement` — Implement one phase at a time
4. `/phase-review` — Post-implementation: prism review, realitycheck, clarion pack, verifier analyze
5. `/commit` when a phase passes all validation

## Architecture

**Four-layer model:**
- **Structural Layer** — files, packages, symbols, declarations, imports
- **Relationship Layer** — calls, implements, imports, embeds, tests, route registrations, config usage, etc.
- **Semantic Layer** — compact structured summaries of files, packages, symbols
- **Freshness Layer** — content hashes, run metadata, invalidation state

**Package layout** (from spec section 19):
- `cmd/atlas/` — CLI entrypoint only
- `internal/app/` — command orchestration
- `internal/cli/` — CLI wiring
- `internal/config/` — config loading from `.atlas/config.yaml`
- `internal/db/` — SQLite migrations and persistence
- `internal/extractor/` — language extractors behind `Extractor` interface
  - `goextractor/` — Go AST-based extraction (priority 1)
  - `tsextractor/` — TypeScript extraction (priority 2)
  - `jsextractor/` — JavaScript extraction (priority 3)
- `internal/indexer/` — file scanning, extraction, invalidation, run orchestration
- `internal/query/` — symbol/file/package lookups and graph traversal
- `internal/summary/` — summary generation and freshness checks
- `internal/model/` — domain types
- `internal/fswalk/` — file walking
- `internal/hash/` — content hashing
- `internal/vcs/` — Git integration
- `internal/diag/` — diagnostics
- `internal/doctor/` — health checks
- `internal/export/` — export commands
- `internal/output/` — text/JSON/agent output formatting

**Key interfaces** (spec section 20): `Extractor`, `IndexService`, `QueryService`, `SummaryService`, `Validator`, `Walker`, `Hasher`, `RepoLocator`

## Storage

All data lives in `.atlas/` under the repo root:
- `atlas.db` — SQLite primary store (12 tables, see spec section 10)
- `config.yaml` — repo-specific configuration
- `manifest.json` — repo identity and schema version

**Freshness rule:** source code is truth. Summaries are derived cache. A summary is stale when its `generated_from_hash` != current file hash. When a file changes: delete its symbols, references, summaries; invalidate dependent package summaries; rebuild from current content.

## Implementation Phases

1. Foundation — CLI skeleton, repo detection, config, SQLite, init/index/stats, file scanning/hashing
2. Go Structural Indexing — Go extractor, files/packages/symbols, imports, tests, queries
3. Relationship Expansion — calls, implementations, routes, config, artifacts
4. Semantic Layer — file/package summaries, stale detection
5. Integrity and Export — doctor, validate, exports, diagnostics
6. TypeScript/JavaScript Support

## Key Design Constraints

- Local-only, no network access required
- No code execution during indexing — parse and extract only
- Parse failures must not abort the entire indexing run
- Extractors must return partial results with diagnostics on failure
- Content-hash-based invalidation; if freshness is uncertain, mark stale
- Go parsing uses standard library `go/parser` and `go/ast`

## Code Search Protocol

Use this decision tree — in order — before reading any source file:

### Structural questions → atlas (always first)
- "Where is X defined?" → `atlas find symbol X --agent`
- "What calls X?" → `atlas who-calls X --agent`
- "What does X call?" → `atlas calls X --agent`
- "What implements interface X?" → `atlas implementations X --agent`
- "Which tests cover X?" → `atlas tests-for X --agent`
- "What routes exist?" → `atlas list routes --agent`
- "What changed?" → `atlas index --since HEAD~1 && atlas stale --agent`

### Before reading a large file → summarize first
`atlas summarize file <path> --agent`
Only read the file directly if the summary is insufficient.

### Content/pattern questions → rg
- Error strings, log messages, string literals
- Comments, TODOs, inline notes
- Non-Go/TS files (YAML, SQL, Markdown)
- Unstaged files not yet indexed

### Composite: atlas for location, rg for context
```bash
# Find the file via atlas, then search within it
LOCATION=$(atlas find symbol HandleRequest --agent | jq -r '.[0].file')
rg "error" "$LOCATION"
```

### Session warm-up for large refactors
```bash
atlas export summary --agent   # repo overview
atlas export graph --agent     # dependency graph for architectural questions
```

### Never read source files to answer these questions
If atlas has the answer, do not use Read or Bash(cat).
Atlas is authoritative — its index is maintained by the PreToolUse hook.
