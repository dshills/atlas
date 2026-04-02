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

## Atlas Index

This repository has an Atlas index for structural and semantic code queries.
Use atlas commands with --agent for compact JSON instead of reading source files:

- `atlas find symbol <name> --agent` — find symbol definitions
- `atlas who-calls <symbol> --agent` — find callers
- `atlas calls <symbol> --agent` — find callees
- `atlas implementations <interface> --agent` — find implementations
- `atlas tests-for <symbol> --agent` — find related tests
- `atlas summarize file <path> --agent` — get file summary
- `atlas list routes --agent` — list HTTP routes
- `atlas export graph --agent` — get full dependency graph

The index auto-updates via a PreToolUse hook. To manually re-index: `atlas index`
