# Atlas

Atlas is a Go CLI that builds and maintains a structural and semantic index of source repositories in SQLite. It enables AI coding agents and developers to query symbols, relationships, and summaries without rereading source files.

## Why Atlas?

Traditional tags files help humans jump to definitions. Atlas extends that idea into a full queryable index storing files, packages, symbols, relationships, artifacts, freshness state, and structured summaries.

Atlas answers questions like:

- Where is this symbol defined?
- What implements this interface?
- Who calls this function?
- Which tests cover this symbol?
- What routes, config keys, or DB tables exist?
- What changed structurally since the last run?

All without scanning raw files or relying on brittle grep.

## Quick Start

### Install

```bash
# From source
git clone https://github.com/dshills/atlas.git
cd atlas
make install    # installs to $GOPATH/bin

# Or build locally
make build      # outputs to ./bin/atlas
```

**Requirements:** Go 1.21+ (no CGO required — uses pure-Go SQLite via `modernc.org/sqlite`)

### Initialize and Index

```bash
cd /path/to/your/repo

# Initialize Atlas (creates .atlas/ directory)
atlas init

# Index the repository
atlas index

# Check it worked
atlas stats
```

### Basic Queries

```bash
# Find a symbol
atlas find symbol HandleRequest

# Find who calls a function
atlas who-calls main.Run

# Find what a function calls
atlas calls server.Start

# Find implementations of an interface
atlas implementations Extractor

# Find tests for a symbol
atlas tests-for pkg.Helper

# List all routes
atlas list routes

# List all packages
atlas list packages
```

## Language Support

| Language | Parser | Symbols | Relationships | Artifacts |
|----------|--------|---------|---------------|-----------|
| Go | `go/parser` + `go/ast` | Full (functions, methods, structs, interfaces, types, consts, vars, fields, tests, benchmarks, entrypoints) | calls, implements, imports, embeds, tests, routes, config, SQL, jobs | routes, config keys, migrations, SQL queries, jobs, external services, env vars |
| TypeScript | Regex/heuristic | Functions, classes, methods, interfaces, types, consts, vars, tests | imports | - |
| JavaScript | Regex/heuristic | Functions, classes, methods, consts, vars, tests | imports | - |

Languages can be individually enabled/disabled in `.atlas/config.yaml`.

## Commands

### Core

| Command | Description |
|---------|-------------|
| `atlas init` | Initialize Atlas in the current repository |
| `atlas index` | Index the repository (incremental by default) |
| `atlas reindex` | Clear and rebuild the entire index |
| `atlas version` | Print version, schema version, and Go version |
| `atlas config` | Print effective configuration |
| `atlas stats` | Show repository statistics |
| `atlas stale` | List stale summaries |

### Querying

| Command | Description |
|---------|-------------|
| `atlas find symbol <name>` | Find symbols by name or qualified name |
| `atlas find file <pattern>` | Find files by path pattern |
| `atlas find package <name>` | Find packages by name or import path |
| `atlas find route <pattern>` | Find route artifacts |
| `atlas find config <key>` | Find config key artifacts |
| `atlas who-calls <symbol>` | Find callers of a symbol |
| `atlas calls <symbol>` | Find outgoing calls from a symbol |
| `atlas implementations <interface>` | Find implementations of an interface |
| `atlas imports <package>` | Find files that import a package |
| `atlas tests-for <target>` | Find tests for a symbol |
| `atlas touches <kind> <name>` | Find references touching an artifact |

### Listing

| Command | Description |
|---------|-------------|
| `atlas list packages` | List all indexed packages |
| `atlas list routes` | List registered HTTP routes |
| `atlas list jobs` | List background jobs and goroutines |
| `atlas list migrations` | List database migrations |
| `atlas list integrations` | List external service integrations |
| `atlas list entrypoints` | List main/entrypoint functions |
| `atlas list diagnostics` | List diagnostics from the latest run |

### Summaries

| Command | Description |
|---------|-------------|
| `atlas summarize file <path>` | Generate/retrieve a file summary |
| `atlas summarize package <name>` | Generate/retrieve a package summary |
| `atlas summarize symbol <qname>` | Generate/retrieve a symbol summary |

Summaries are deterministic aggregations of indexed data (not LLM-generated). They include responsibilities, dependencies, public API surface, side effects, and related artifacts.

### Health and Integrity

| Command | Description |
|---------|-------------|
| `atlas doctor` | Run 8 health checks (storage, DB, schema, manifest, runs, stale summaries, missing files, SQLite integrity) |
| `atlas validate` | Run 12 integrity checks (FK violations, orphaned symbols, stale summaries, denormalized field consistency, length constraints) |
| `atlas validate --strict` | Additionally check that all files matching include globs are indexed |
| `atlas hook install` | Install Claude Code PreToolUse hook for automatic re-indexing (`--claude-md` to also write CLAUDE.md instructions) |
| `atlas hook uninstall` | Remove the Claude Code hook |
| `atlas hook status` | Check if the hook is installed |

### Export

All export commands produce stable JSON schemas. Use `--out <file>` to write to a file instead of stdout.

| Command | Description |
|---------|-------------|
| `atlas export summary` | Repository overview (packages, entrypoints, stale counts, diagnostics) |
| `atlas export graph` | Symbol/reference graph as nodes and edges |
| `atlas export symbols` | All symbols with qualified names, kinds, locations |
| `atlas export packages` | All packages with file counts |
| `atlas export routes` | All route artifacts with method, path, handler |
| `atlas export diagnostics` | Diagnostics from the latest run |

## Global Flags

| Flag | Description |
|------|-------------|
| `--repo <path>` | Explicit repository root path (overrides auto-detection) |
| `--json` | Output in indented JSON format |
| `--agent` | Output in compact JSON (optimized for machine consumption) |

## Incremental Indexing

Atlas uses content-hash-based invalidation. On each run:

1. Walk files matching include/exclude globs
2. Compute SHA-256 hash of each file
3. Skip files whose hash matches the stored hash
4. For changed files, run an 8-step invalidation cascade:
   - Delete old symbols and outgoing references
   - Mark incoming references as unresolved
   - Delete stale summaries
   - Re-extract and persist new data
   - Re-resolve cross-file references

```bash
# Full index (default)
atlas index

# Only files changed since a git revision
atlas index --since HEAD~5
atlas index --since abc1234

# Fail on any extraction errors
atlas index --strict
```

## Configuration

Atlas stores configuration in `.atlas/config.yaml`, created by `atlas init`:

```yaml
version: 1

include:
  - "**/*.go"
  - "**/*.ts"
  - "**/*.tsx"
  - "**/*.js"
  - "**/*.jsx"

exclude:
  - "vendor/**"
  - "node_modules/**"
  - ".git/**"
  - "testdata/**"

languages:
  go: true
  typescript: true
  javascript: true

indexing:
  max_file_size_bytes: 1048576    # 1 MiB

summaries:
  enabled: true
  file: true
  package: true
  symbol: true

output:
  default_format: text
```

### Repository Root Detection

Atlas finds the repository root in this order:
1. `--repo` flag (explicit)
2. `repo_root` in config
3. Git root (walks up looking for `.git/`)
4. Current working directory

## Storage

All data lives in `.atlas/` under the repository root:

```
.atlas/
├── atlas.db        # SQLite database (12 tables)
├── config.yaml     # Repository configuration
└── manifest.json   # Repo identity and schema version
```

The database uses WAL mode with foreign keys enabled. No external services or network access required.

## Integrating with AI Coding Agents

Atlas is designed to be a backend for AI coding agents. The `--agent` flag produces compact JSON optimized for LLM context windows.

### Agent Setup

```bash
# One-time setup in a repository
atlas init && atlas index

# Install Claude Code hook for automatic re-indexing
atlas hook install
```

The `atlas hook install` command adds a `PreToolUse` hook to `.claude/settings.json` that runs `atlas index` before each Bash command. This keeps the index fresh as Claude makes changes — no manual re-indexing needed.

Use `--claude-md` to also append Atlas usage instructions to your project's `CLAUDE.md`:

```bash
atlas hook install --claude-md   # Install hook + write CLAUDE.md instructions
```

```bash
atlas hook status      # Check if hook is installed
atlas hook uninstall   # Remove the hook
```

### Recommended Agent Workflow

Before making changes, an agent should query Atlas to understand the codebase:

```bash
# Find where a symbol is defined
atlas find symbol UserService --agent

# Understand the call graph before modifying a function
atlas who-calls HandleRequest --agent
atlas calls HandleRequest --agent

# Find what tests to run after a change
atlas tests-for pkg.MyFunction --agent

# Check what implements an interface before adding methods
atlas implementations Repository --agent

# Get a file summary instead of reading the whole file
atlas summarize file internal/server/handler.go --agent

# Find all routes to understand the API surface
atlas list routes --agent

# After making changes, re-index incrementally
atlas index --since HEAD~1
```

### Adding Atlas to Agent Instructions

Add to your `CLAUDE.md`, `.cursorrules`, or agent system prompt:

```markdown
## Atlas Index

This repository has an Atlas index. Use `atlas` commands with `--agent` for
compact JSON output instead of reading source files directly:

- `atlas find symbol <name> --agent` — find symbol definitions
- `atlas who-calls <symbol> --agent` — find callers
- `atlas calls <symbol> --agent` — find callees
- `atlas implementations <interface> --agent` — find implementations
- `atlas tests-for <symbol> --agent` — find related tests
- `atlas summarize file <path> --agent` — get file summary
- `atlas list routes --agent` — list HTTP routes
- `atlas export graph --agent` — get full dependency graph

Run `atlas index` after making changes to keep the index fresh.
```

### Export for Context Loading

Use export commands to load structured context into an agent's prompt:

```bash
# Full repository overview for initial context
atlas export summary > context/repo-summary.json

# Dependency graph for architectural questions
atlas export graph > context/graph.json

# All symbols for codebase-wide queries
atlas export symbols > context/symbols.json
```

### Keeping the Index Fresh

```bash
# Re-index after changes (fast — only processes changed files)
atlas index

# Verify index health
atlas doctor --agent

# Check for stale summaries
atlas stale --agent
```

## Architecture

Atlas uses a four-layer model:

| Layer | Purpose | Data |
|-------|---------|------|
| **Structural** | What exists | Files, packages, symbols, declarations, imports |
| **Relationship** | How things connect | Calls, implements, imports, embeds, tests, routes, config |
| **Semantic** | What things mean | File/package/symbol summaries, responsibilities, public API |
| **Freshness** | What's current | Content hashes, run metadata, invalidation state |

### Database Schema

12 tables with CHECK constraints, cascading deletes, and indexes:

- `files` — source file metadata with content hashes
- `packages` — package/module definitions
- `package_files` — file-to-package associations
- `symbols` — all symbol definitions (17 kinds)
- `references` — relationships between symbols (14 kinds)
- `file_summaries` — cached file summaries
- `package_summaries` — cached package summaries
- `symbol_summaries` — cached symbol summaries
- `artifacts` — higher-level constructs (routes, config keys, migrations, etc.)
- `index_runs` — indexing operation history
- `diagnostics` — warnings and errors from indexing
- `schema_meta` — schema version tracking

## Development

```bash
make help       # Show all targets
make build      # Build to ./bin/atlas
make test       # Run all tests
make test-race  # Run all tests with race detector
make lint       # Run golangci-lint
make install    # Install to $GOPATH/bin
make clean      # Remove build artifacts
```

**Linter setup:**
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Running a Single Test

```bash
go test ./internal/indexer -run TestIncrementalOnlyReindexesChanged
```

### Dependencies

- [cobra](https://github.com/spf13/cobra) — CLI framework
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure-Go SQLite (no CGO)
- [doublestar](https://github.com/bmatcuk/doublestar) — glob pattern matching
- [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) — YAML configuration parsing

## License

See [LICENSE](LICENSE) for details.
