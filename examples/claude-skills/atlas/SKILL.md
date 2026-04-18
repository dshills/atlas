# Atlas Code Search Skill

Atlas is a Go CLI that builds and maintains a structural and semantic index of a repository in SQLite.
Use it as your **primary navigation layer** before touching source files.
The index is kept fresh automatically via a `PreToolUse` hook that runs `atlas index` before each Bash command.

---

## Decision Tree — use this order every time

### 1. Structural questions → atlas (always first)

| Question | Command |
|---|---|
| Where is X defined? | `atlas find symbol X --agent` |
| What calls X? | `atlas who-calls X --agent` |
| What does X call? | `atlas calls X --agent` |
| What implements interface X? | `atlas implementations X --agent` |
| Which tests cover X? | `atlas tests-for X --agent` |
| What routes exist? | `atlas list routes --agent` |
| What background jobs exist? | `atlas list jobs --agent` |
| What DB migrations exist? | `atlas list migrations --agent` |
| What external integrations exist? | `atlas list integrations --agent` |
| What are the entrypoints? | `atlas list entrypoints --agent` |
| What packages are indexed? | `atlas list packages --agent` |
| What changed recently? | `atlas index --since HEAD~1 && atlas stale --agent` |

### 2. Before reading a large file → summarize first

```
atlas summarize file <path> --agent
atlas summarize package <name> --agent
atlas summarize symbol <qualified.Name> --agent
```

Only read the file directly if the summary is insufficient for the task.

### 3. Content/pattern questions → rg

Use `rg` (ripgrep) for:
- Error strings, log messages, string literals
- Comments, TODOs, inline notes
- Non-indexed file types (YAML, SQL, Markdown, proto)
- Unstaged new files not yet in the index

### Hard rule

**If atlas can answer it, do not use Read or `cat`.**
Atlas is authoritative. Its index is maintained by the PostToolUse hook on every Write/Edit/MultiEdit.

---

## Full Command Reference

### Core / Setup

```bash
atlas init                    # Initialize .atlas/ in repo root
atlas index                   # Incremental re-index (hash-based, fast)
atlas index --since HEAD~5    # Only files changed since git revision
atlas reindex                 # Full wipe and rebuild
atlas stats                   # Repository statistics
atlas stale --agent           # List symbols/files with stale summaries
atlas config                  # Print effective configuration
atlas version                 # Print version info
```

### Symbol & Relationship Queries

```bash
atlas find symbol <name> --agent           # Definition lookup by name or qualified name
atlas find file <pattern> --agent          # File path pattern search
atlas find package <name> --agent          # Package by name or import path
atlas find route <pattern> --agent         # Route artifact search
atlas find config <key> --agent            # Config key artifact search

atlas who-calls <symbol> --agent           # Callers of a function/method
atlas calls <symbol> --agent               # Callees of a function/method
atlas implementations <interface> --agent  # Types implementing an interface
atlas imports <package> --agent            # Files that import a package
atlas tests-for <symbol> --agent           # Test functions covering a symbol
atlas touches <kind> <name> --agent        # References touching an artifact
```

### List Commands

```bash
atlas list routes --agent          # All HTTP routes (method + path + handler)
atlas list jobs --agent            # Background goroutines and jobs
atlas list migrations --agent      # DB migration files
atlas list integrations --agent    # External service integrations
atlas list entrypoints --agent     # main() and entrypoint functions
atlas list packages --agent        # All indexed packages
atlas list diagnostics --agent     # Warnings/errors from last index run
```

### Summaries (structural aggregations, not LLM-generated)

```bash
atlas summarize file <path> --agent           # Responsibilities, deps, public API, side effects
atlas summarize package <name> --agent        # Package-level summary
atlas summarize symbol <qualified.Name> --agent
```

### Export (for bulk context loading)

```bash
atlas export summary --agent    # Repo overview: packages, entrypoints, stale counts
atlas export graph --agent      # Full symbol/reference graph (nodes + edges)
atlas export symbols --agent    # All symbols with kinds and locations
atlas export packages --agent   # All packages with file counts
atlas export routes --agent     # All routes with method/path/handler
atlas export diagnostics --agent
```

Add `--out <file>` to any export to write JSON to disk instead of stdout.

### Health Checks

```bash
atlas doctor --agent      # 8 health checks (storage, DB, schema, stale, integrity)
atlas validate --agent    # 12 integrity checks (FK violations, orphans, denorm consistency)
atlas validate --strict   # Also verify all glob-matched files are indexed
```

### Hook Management

```bash
atlas hook install         # Add PreToolUse hook to .claude/settings.json
atlas hook install --claude-md  # Hook + append instructions to CLAUDE.md
atlas hook uninstall
atlas hook status
```

---

## Output Flags

| Flag | Use for |
|---|---|
| `--agent` | Compact JSON — always use for agent queries |
| `--json` | Indented JSON — use when debugging or exporting |
| *(none)* | Human-readable text — avoid in agent context |

---

## Language Coverage

Atlas indexes: **Go** (full AST), **TypeScript**, **JavaScript**, **Python**, **Rust**, **Java**, **C#**, **Swift**, **Lua** (all regex/heuristic).

Go gets exact AST-level confidence for all relationship extraction.
All languages support: symbols, calls, imports, tests, routes, config keys, SQL, migrations, external services, background jobs.

---

## Index Freshness

- The `PreToolUse` hook runs `atlas index` automatically before each Bash command.
- Content-hash-based: only changed files are re-processed.
- After large refactors, run `atlas reindex` to rebuild from scratch.
- Run `atlas doctor --agent` if queries return unexpected results.

---

## Workflow Patterns

### Before modifying a function

```bash
atlas who-calls HandleRequest --agent    # Who will be affected
atlas calls HandleRequest --agent        # What it depends on
atlas tests-for HandleRequest --agent    # Tests to run after change
```

### Before adding a method to an interface

```bash
atlas implementations Repository --agent  # All types that must be updated
```

### Understanding a file before editing

```bash
atlas summarize file internal/server/handler.go --agent
# Read directly only if the summary leaves questions unanswered
```

### After a large change

```bash
atlas index                  # Incremental refresh
atlas stale --agent          # Any summaries invalidated
atlas validate --agent       # Integrity check
```

### Initial context loading for a new task

```bash
atlas export summary --agent   # Repo shape and entrypoints
atlas list routes --agent      # API surface
atlas list packages --agent    # Package topology
```
