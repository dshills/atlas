# PLAN.md — Regex-Based Relationship Extraction for TS/Python/Rust

## Overview

This plan implements SPEC.md sections 3–12, adding regex-based relationship and artifact extraction to the TypeScript, Python, and Rust extractors. The work is split into five phases: a shared foundation (comment filter), then three language-parallel phases grouped by extraction type (routes+config+tests, SQL+services, calls), and a final integration phase.

Each phase is independently testable and produces a `go test ./... && golangci-lint run ./...` clean result.

---

## Phase 1: Comment Filter Utility

**Goal:** Build the shared `commentfilter` package that all subsequent extraction functions depend on to skip matches inside comments.

**Spec sections:** 7.1–7.3

### Files to Create

| File | Purpose |
|---|---|
| `internal/extractor/commentfilter/commentfilter.go` | `LineFilter(content, lang string) []bool` implementation |
| `internal/extractor/commentfilter/commentfilter_test.go` | Unit tests for all three language modes |

### Implementation Details

- `LineFilter` returns a `[]bool` where `true` = code line, `false` = comment/block-comment line.
- **TypeScript/JS mode:** Track `//` single-line comments and `/* */` block comments. A line is non-code if it starts with `//` (after whitespace) or is inside a `/* */` block.
- **Python mode:** Track `#` single-line comments and `"""`/`'''` docstring blocks. Lines inside docstring blocks are marked non-code. Lines where the first non-whitespace is `#` are marked non-code.
- **Rust mode:** Track `//` single-line comments and `/* */` nestable block comments. Maintain a nesting depth counter; lines at depth > 0 are non-code.
- The filter operates line-by-line in a single pass (O(n) in lines).
- Lines that contain both code and a trailing comment (e.g., `x := 1 // comment`) are marked as **code** — the extraction regex may match the code portion, which is acceptable.

### Acceptance Criteria

1. `LineFilter(content, "typescript")` correctly marks `//` and `/* */` comment lines.
2. `LineFilter(content, "python")` correctly marks `#` and triple-quote block lines.
3. `LineFilter(content, "rust")` correctly marks `//` and nested `/* */` lines.
4. Edge cases tested: empty file, file with only comments, block comment spanning entire file, nested Rust block comments, triple-quote strings in Python.
5. `go test ./internal/extractor/commentfilter/...` passes.
6. `golangci-lint run ./...` passes.

### Risks

- Triple-quote blocks in Python are ambiguous (string literal vs docstring). Accepted as a known limitation per spec section 7.2.
- Comment starts inside string literals will cause false non-code marking. Accepted per spec section 12.2.

---

## Phase 2: Routes + Config + Test References (All Three Languages)

**Goal:** Add the three highest-value, lowest-risk extraction types to all three extractors. These patterns are distinctive (low false-positive rate) and produce both references and artifacts.

**Spec sections:** 4.1–4.2, 4.4, 5.1–5.2, 5.4, 6.1–6.2, 6.4

### Files to Create

| File | Purpose |
|---|---|
| `internal/extractor/tsextractor/routes.go` | TS/JS route extraction (Express, NestJS, Next.js) |
| `internal/extractor/tsextractor/routes_test.go` | Unit tests |
| `internal/extractor/tsextractor/config.go` | TS/JS config/env extraction (process.env, config.get) |
| `internal/extractor/tsextractor/config_test.go` | Unit tests |
| `internal/extractor/tsextractor/tests.go` | TS/JS test reference extraction (describe blocks) |
| `internal/extractor/tsextractor/tests_test.go` | Unit tests |
| `internal/extractor/tsextractor/testdata/routes.ts` | Test fixture: route patterns |
| `internal/extractor/tsextractor/testdata/config.ts` | Test fixture: config patterns |
| `internal/extractor/pyextractor/routes.go` | Python route extraction (Flask, FastAPI, Django) |
| `internal/extractor/pyextractor/routes_test.go` | Unit tests |
| `internal/extractor/pyextractor/config.go` | Python config/env extraction (os.environ, os.getenv, settings) |
| `internal/extractor/pyextractor/config_test.go` | Unit tests |
| `internal/extractor/pyextractor/tests.go` | Python test reference extraction (test_ prefix) |
| `internal/extractor/pyextractor/tests_test.go` | Unit tests |
| `internal/extractor/pyextractor/testdata/routes.py` | Test fixture: route patterns |
| `internal/extractor/pyextractor/testdata/config.py` | Test fixture: config patterns |
| `internal/extractor/rustextractor/routes.go` | Rust route extraction (Actix, Rocket, Axum) |
| `internal/extractor/rustextractor/routes_test.go` | Unit tests |
| `internal/extractor/rustextractor/config.go` | Rust config/env extraction (env::var, dotenv, config crate) |
| `internal/extractor/rustextractor/config_test.go` | Unit tests |
| `internal/extractor/rustextractor/tests.go` | Rust test reference extraction (test_ prefix) |
| `internal/extractor/rustextractor/tests_test.go` | Unit tests |
| `internal/extractor/rustextractor/testdata/routes.rs` | Test fixture: route patterns |
| `internal/extractor/rustextractor/testdata/config.rs` | Test fixture: config patterns |

### Files to Modify

| File | Change |
|---|---|
| `internal/extractor/tsextractor/ts.go` | Wire `extractRoutes`, `extractConfigAccess`, `extractTestReferences` into `Extract()` after line 68. Pass comment filter. |
| `internal/extractor/pyextractor/py.go` | Wire `extractRoutes`, `extractConfigAccess`, `extractTestReferences` into `Extract()` after line 53. Pass comment filter. |
| `internal/extractor/rustextractor/rust.go` | Wire `extractRoutes`, `extractConfigAccess`, `extractTestReferences` into `Extract()` after line 49. Pass comment filter. |

### Implementation Details

**Function signatures** (consistent across all three extractors):

```go
func extractRoutes(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
func extractConfigAccess(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
func extractTestReferences(symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord
```

**Comment filter integration:** Each extractor's `Extract()` computes `codeLines := commentfilter.LineFilter(content, lang)` once and passes it to all extraction functions. Extraction functions skip matches where `codeLines[lineNum-1] == false`.

**Route extraction per language:**
- **TypeScript:** Three regexes — `expressRouteRe`, `nestDecoratorRe`, `nextRouteExportRe` (spec 4.1). Produces `registers_route` references + `route` artifacts.
- **Python:** Two regexes — `decoratorRouteRe` (Flask/FastAPI), `djangoPathRe` (spec 5.1). The `route` method maps to HTTP method `ANY`.
- **Rust:** Three regexes — `attrRouteRe` (Actix/Rocket), `axumRouteRe`, `actixRouteRe` (spec 6.1). Attribute macros are `exact` confidence; builder patterns are `heuristic`.

**Config extraction per language:**
- **TypeScript:** Three regexes — `processEnvDotRe`, `processEnvBrackRe`, `configGetRe` (spec 4.2). All `exact` confidence.
- **Python:** Four regexes — `osEnvironBrackRe`, `osEnvironGetRe`, `osGetenvRe`, `settingsDotRe` (spec 5.2). All `exact` confidence. Note: `settingsDotRe` matches only `UPPER_CASE` keys per Django convention (`settings.DEBUG`, `settings.SECRET_KEY`). Django settings are conventionally all-uppercase; lowercase attribute access on `settings` is almost always a method or module, not a config key.
- **Rust:** Three regexes — `envVarRe`, `dotenvVarRe`, `configGetRe` (spec 6.2). All `exact` confidence.

**Test reference extraction per language:**
- **TypeScript:** Match `describe('Name'` blocks against extracted symbols. Strip `test`/`Test` prefix from named test functions and match (spec 4.4).
- **Python:** Strip `test_` prefix from test function names, match against extracted symbols case-insensitively. Strip `Test` prefix from test class names (spec 5.4).
- **Rust:** Strip `test_` prefix from test function names, match against extracted symbols (spec 6.4).

**All regexes** compiled at package init (`var re = regexp.MustCompile(...)`) per spec section 10.1.

**Artifact DataJSON** produced using `encoding/json.Marshal` on small structs or maps. This ensures proper escaping of special characters (quotes, backslashes, newlines) in route paths, SQL queries, and config keys. Do not use `fmt.Sprintf` for JSON construction.

### Key Decisions

1. The comment filter is computed once in `Extract()` and threaded through — not recomputed per extraction function.
2. Test reference extraction does not need the comment filter because it operates on already-extracted symbol records, not raw content.
3. `extractTestReferences` takes the symbol list as input and returns only references — it does not produce artifacts.
4. Route handler names are extracted on a best-effort basis. If the handler cannot be identified (e.g., inline function), the handler field is empty.
5. **`extractTestCalls` vs `extractTestReferences` (TS only):** The existing `extractTestCalls` in `tsextractor/ts.go` extracts `describe`/`it`/`test` blocks as **SymbolRecord** entries (symbol kind `"test"`). The new `extractTestReferences` generates **ReferenceRecord** entries (reference kind `"tests"`) linking those test symbols back to the symbols they test. They are complementary — `extractTestCalls` produces test symbols, `extractTestReferences` consumes them to produce cross-references. Both remain in the pipeline.
6. **Spec signature alignment:** The spec section 3.2 signatures have been updated to include the `codeLines []bool` parameter, matching the plan's function signatures.

### Acceptance Criteria

1. `atlas index` on a TypeScript project with Express routes produces `route` artifacts visible in `atlas list routes --agent`.
2. `atlas index` on a Python project with Flask/FastAPI decorators produces `route` artifacts.
3. `atlas index` on a Rust project with Actix `#[get("/")]` produces `route` artifacts.
4. `process.env.KEY`, `os.getenv('KEY')`, and `env::var("KEY")` produce `env_var` artifacts.
5. Test functions in all three languages produce `tests` references linking to the tested symbol.
6. Patterns inside comments do not produce references or artifacts.
7. All unit tests pass: `go test ./internal/extractor/...`
8. `golangci-lint run ./...` passes.

### Risks

- Flask and FastAPI decorator regexes overlap (both use `@var.method('/path')`). This is intentional — the extraction is framework-agnostic.
- NestJS decorator patterns (`@Get()`) may conflict with other decorators. Mitigated by checking the decorator name is a known HTTP method.

---

## Phase 3: SQL Artifacts + External Services + Background Jobs (All Three Languages)

**Goal:** Add SQL query/migration detection and external service/background job detection to all three extractors.

**Spec sections:** 4.3, 4.5, 5.3, 5.5, 6.3, 6.5

### Files to Create

| File | Purpose |
|---|---|
| `internal/extractor/tsextractor/sql.go` | TS/JS SQL artifact extraction |
| `internal/extractor/tsextractor/sql_test.go` | Unit tests |
| `internal/extractor/tsextractor/services.go` | TS/JS external service + background job detection |
| `internal/extractor/tsextractor/services_test.go` | Unit tests |
| `internal/extractor/tsextractor/testdata/sql.ts` | Test fixture: SQL patterns |
| `internal/extractor/tsextractor/testdata/services.ts` | Test fixture: service/job patterns |
| `internal/extractor/pyextractor/sql.go` | Python SQL artifact extraction |
| `internal/extractor/pyextractor/sql_test.go` | Unit tests |
| `internal/extractor/pyextractor/services.go` | Python external service + background job detection |
| `internal/extractor/pyextractor/services_test.go` | Unit tests |
| `internal/extractor/pyextractor/testdata/sql.py` | Test fixture: SQL patterns |
| `internal/extractor/pyextractor/testdata/services.py` | Test fixture: service/job patterns |
| `internal/extractor/rustextractor/sql.go` | Rust SQL artifact extraction |
| `internal/extractor/rustextractor/sql_test.go` | Unit tests |
| `internal/extractor/rustextractor/services.go` | Rust external service + background job detection |
| `internal/extractor/rustextractor/services_test.go` | Unit tests |
| `internal/extractor/rustextractor/testdata/sql.rs` | Test fixture: SQL patterns |
| `internal/extractor/rustextractor/testdata/services.rs` | Test fixture: service/job patterns |

### Files to Modify

| File | Change |
|---|---|
| `internal/extractor/tsextractor/ts.go` | Wire `extractSQLArtifacts`, `extractServices` into `Extract()`. |
| `internal/extractor/pyextractor/py.go` | Wire `extractSQLArtifacts`, `extractServices` into `Extract()`. |
| `internal/extractor/rustextractor/rust.go` | Wire `extractSQLArtifacts`, `extractServices` into `Extract()`. |

### Implementation Details

**Function signatures:**

```go
func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
func extractServices(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
```

**SQL extraction approach:**
1. Check if `filePath` contains `migrations/` or `migrate/` → emit `migration` artifact + `migrates` reference.
2. Extract string literal contents using language-appropriate regexes:
   - **TypeScript:** Single/double-quoted strings (20+ chars) and template literals.
   - **Python:** Single/double-quoted strings, triple-quoted strings, f-strings.
   - **Rust:** Regular strings, raw strings (`r"..."`, `r#"..."#`), `sqlx::query!("...")`.
3. Apply `sqlKeywordRe` to each extracted string. If match → emit `sql_query` artifact + `touches_table` reference.
4. Truncate query in DataJSON to 200 chars. Truncate `RawTargetText` to 100 chars.
5. All SQL detection uses `heuristic` confidence.

**Service extraction per language:**
- **TypeScript:** `fetch`, `axios.*`, `http.get/post/request` → `external_service` artifact + `invokes_external_api` reference. `new Worker(`, queue patterns → `background_job` artifact (no reference).
- **Python:** `requests.*`, `httpx.*`, `urllib.request.urlopen`, `aiohttp.ClientSession` → `external_service`. `@app.task`/`@shared_task` (Celery), `threading.Thread`, `asyncio.create_task`, `subprocess.*` → `background_job`.
- **Rust:** `reqwest::*`, `hyper::Client`, `tonic::*` → `external_service`. `tokio::spawn`, `thread::spawn`, `.par_iter()` → `background_job`.

**Key difference from Go extractor:** The Go extractor detects goroutines via AST (`ast.GoStmt`). The regex extractors match language-equivalent patterns (Workers, Celery tasks, tokio::spawn) instead.

### Key Decisions

1. SQL string extraction uses a minimum length threshold (20 chars) to avoid matching short strings that happen to start with SQL keywords.
2. External service detection only emits `invokes_external_api` references for HTTP/RPC clients, not background jobs. This matches Go extractor behavior.
3. Background job artifacts do not produce references (same as Go extractor — goroutines produce artifacts only).

### Acceptance Criteria

1. A TypeScript file with `` `SELECT * FROM users` `` produces a `sql_query` artifact.
2. A Python migration file in `migrations/` produces a `migration` artifact + `migrates` reference.
3. `requests.get('https://api.example.com')` produces an `external_service` artifact + `invokes_external_api` reference.
4. `tokio::spawn(async { ... })` produces a `background_job` artifact.
5. SQL keywords inside comments are not extracted.
6. All unit tests pass: `go test ./internal/extractor/...`
7. `golangci-lint run ./...` passes.

### Risks

- Template literal SQL in TypeScript may span multiple lines. The `(?s)` flag handles this, but regex may be greedy. Use non-greedy `*?` quantifiers.
- Python f-string SQL (`f"SELECT {col}"`) has interpolation. The query string in the artifact will contain the `{col}` placeholder — acceptable for heuristic detection.

---

## Phase 4: Call Extraction (All Three Languages)

**Goal:** Add heuristic call detection to all three extractors. This is the highest-value but noisiest extraction type.

**Spec sections:** 4.6, 5.6, 6.6

### Files to Create

| File | Purpose |
|---|---|
| `internal/extractor/tsextractor/calls.go` | TS/JS call extraction |
| `internal/extractor/tsextractor/calls_test.go` | Unit tests |
| `internal/extractor/tsextractor/testdata/calls.ts` | Test fixture: call patterns |
| `internal/extractor/pyextractor/calls.go` | Python call extraction |
| `internal/extractor/pyextractor/calls_test.go` | Unit tests |
| `internal/extractor/pyextractor/testdata/calls.py` | Test fixture: call patterns |
| `internal/extractor/rustextractor/calls.go` | Rust call extraction |
| `internal/extractor/rustextractor/calls_test.go` | Unit tests |
| `internal/extractor/rustextractor/testdata/calls.rs` | Test fixture: call patterns |

### Files to Modify

| File | Change |
|---|---|
| `internal/extractor/tsextractor/ts.go` | Wire `extractCalls` into `Extract()`. |
| `internal/extractor/pyextractor/py.go` | Wire `extractCalls` into `Extract()`. |
| `internal/extractor/rustextractor/rust.go` | Wire `extractCalls` into `Extract()`. |

### Implementation Details

**Function signature:**

```go
func extractCalls(content string, lines []string, codeLines []bool, symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord
```

**TypeScript/JS call extraction:**
1. Apply `directCallRe` (`(?m)(?:^|[^.\w])([a-z]\w*)\s*\(`) to content.
2. Apply `selectorCallRe` (`(?m)(\w+)\.(\w+)\s*\(`) to content.
3. For each match: compute line number, check `codeLines[line-1] == true`, check identifier is not in keyword exclusion set.
4. **Keyword exclusion set:** `function`, `class`, `if`, `for`, `while`, `switch`, `return`, `import`, `export`, `new`, `catch`, `typeof`, `instanceof`, `case`, `throw`, `await`, `yield`, `from`, `of`, `else`, `do`, `try`, `finally`, `const`, `let`, `var`, `delete`, `void`, `in`.
5. Produce `ReferenceRecord` with `ReferenceKind: "calls"`, `Confidence: "heuristic"`, `RawTargetText: identifier` or `receiver.method`.

**Python call extraction:**
1. Same regex approach adapted for Python identifiers (`[a-z_]\w*`).
2. **Keyword exclusion set:** `def`, `class`, `if`, `elif`, `for`, `while`, `with`, `except`, `import`, `from`, `as`, `and`, `or`, `not`, `in`, `is`, `lambda`, `assert`, `raise`, `return`, `yield`, `del`, `global`, `nonlocal`, `pass`, `break`, `continue`, `try`, `finally`, `else`.
3. **Builtin exclusion set:** `print`, `len`, `range`, `str`, `int`, `float`, `bool`, `list`, `dict`, `set`, `tuple`, `type`, `super`, `isinstance`, `issubclass`, `hasattr`, `getattr`, `setattr`, `delattr`, `repr`, `hash`, `id`, `abs`, `min`, `max`, `sum`, `sorted`, `reversed`, `enumerate`, `zip`, `map`, `filter`, `open`, `input`, `iter`, `next`, `any`, `all`, `dir`, `vars`, `globals`, `locals`.

**Rust call extraction:**
1. Four regexes: `directCallRe`, `pathCallRe` (`(\w+(?:::\w+)+)\s*\(`), `selectorCallRe`, `macroCallRe` (`(\w+)!\s*[\(\[\{]`).
2. **Keyword exclusion set:** `fn`, `let`, `mut`, `const`, `static`, `struct`, `enum`, `trait`, `type`, `impl`, `mod`, `use`, `pub`, `if`, `else`, `for`, `while`, `loop`, `match`, `return`, `where`, `as`, `in`, `ref`, `move`, `break`, `continue`, `unsafe`, `async`, `await`, `dyn`, `extern`, `crate`, `self`, `super`.
3. **Macro exclusion set:** `println`, `eprintln`, `dbg`, `format`, `vec`, `assert`, `assert_eq`, `assert_ne`, `cfg`, `derive`, `todo`, `unimplemented`, `unreachable`, `panic`, `write`, `writeln`, `include`, `include_str`, `include_bytes`, `env`, `concat`, `stringify`, `line`, `file`, `column`, `module_path`.

**Deduplication:** Each extraction function maintains a `map[string]bool` keyed on `"line:target"` to avoid emitting duplicate references for the same call on the same line (e.g., `selectorCallRe` matching a subset of what `directCallRe` matched).

### Key Decisions

1. Call extraction runs **after** route, config, SQL, and service extraction. Calls to framework functions already captured as routes/config/services are not excluded — they will appear as both a specific reference (e.g., `registers_route`) and a generic `calls` reference. This is acceptable because consumers filter by reference kind.
2. The `symbols` parameter is accepted but not used for scope resolution in this phase. It is included in the signature for forward compatibility if a future phase adds scope-aware filtering.
3. Builtins are excluded for Python to reduce noise. TypeScript builtins (`console`, `JSON`, `Math`, `Object`, `Array`, `Promise`) are NOT excluded because they represent meaningful calls in the context of code understanding.

### Acceptance Criteria

1. `processData(x)` in TypeScript produces a `calls` reference with `RawTargetText: "processData"`.
2. `service.getData()` in Python produces a `calls` reference with `RawTargetText: "service.getData"`.
3. `std::fs::read_to_string(path)` in Rust produces a `calls` reference with `RawTargetText: "std::fs::read_to_string"`.
4. `if (condition)` does NOT produce a call reference in any language.
5. `println!("hello")` does NOT produce a call reference in Rust.
6. `print("hello")` does NOT produce a call reference in Python.
7. Calls inside comments are not extracted.
8. No duplicate references for the same call on the same line.
9. All unit tests pass: `go test ./internal/extractor/...`
10. `golangci-lint run ./...` passes.

### Risks

- **False positive volume:** Call extraction will produce more references than all other extraction types combined. This is expected and mitigated by `heuristic` confidence. Downstream query commands should allow confidence-based filtering.
- **Regex overlap:** `selectorCallRe` may match the same call as `directCallRe` (e.g., `foo.bar(` matches both). Deduplication by line+target handles this.
- **Decorator/attribute calls in Python/Rust:** `@decorator` and `#[attr]` look like calls to the regex. The keyword/builtin exclusion sets do not cover all decorators. Accepted — `heuristic` confidence signals this to consumers.

---

## Phase 5: Integration Testing + Extract() Wiring Cleanup

**Goal:** Ensure all extraction types work together end-to-end through `atlas index` and verify query commands return cross-language results.

**Spec sections:** 8.1–8.3, 9.2–9.3

### Files to Create

| File | Purpose |
|---|---|
| `internal/extractor/tsextractor/testdata/full.ts` | Integration fixture: all pattern types combined |
| `internal/extractor/pyextractor/testdata/full.py` | Integration fixture: all pattern types combined |
| `internal/extractor/rustextractor/testdata/full.rs` | Integration fixture: all pattern types combined |

### Files to Modify

| File | Change |
|---|---|
| `internal/extractor/tsextractor/ts_test.go` | Add integration test calling `Extract()` on `testdata/full.ts`, verifying all reference and artifact types present. |
| `internal/extractor/pyextractor/py_test.go` | Add integration test calling `Extract()` on `testdata/full.py`. |
| `internal/extractor/rustextractor/rust_test.go` | Add integration test calling `Extract()` on `testdata/full.rs`. |

### Implementation Details

**Integration test fixtures** each contain:
- A route registration (framework-specific)
- An environment variable access
- A config key access
- A SQL query in a string literal
- A test function referencing a symbol
- An external HTTP client call
- A background job/task pattern
- Several function/method calls
- Comments containing patterns that should NOT be extracted

**Integration test assertions** verify:
- Correct count of each reference kind
- Correct count of each artifact kind
- No references from comment lines
- Confidence values match expected (exact vs heuristic)
- DataJSON is valid JSON for all artifacts

**Extract() wiring review:** Confirm that each extractor's `Extract()` method calls all six extraction functions in the correct order:
1. `extractImports` (existing)
2. `extractSymbols` (existing)
3. `extractTestCalls` (existing, TS only)
4. Compute `codeLines` via `commentfilter.LineFilter`
5. `extractRoutes`
6. `extractConfigAccess`
7. `extractTestReferences`
8. `extractSQLArtifacts`
9. `extractServices`
10. `extractCalls`

### Acceptance Criteria

1. Integration tests pass for all three languages with all reference and artifact types present.
2. `go test ./internal/extractor/...` passes (all unit + integration tests).
3. `golangci-lint run ./...` passes.
4. `atlas index` on the atlas repo itself (Go files) still works correctly — no regressions.
5. Each extractor's `SupportedKinds()` return value remains unchanged (extraction types don't add new symbol kinds).

### Risks

- Integration fixtures may not cover all edge case combinations. Mitigated by the extensive unit tests in phases 2–4.
- Performance regression from six additional extraction passes per file. Mitigated by spec section 10.2 expectation of < 5% overhead.

---

## Performance Verification

To ensure the < 5% indexing overhead target (spec section 10.2) is met:

1. **Baseline measurement:** Before starting Phase 2, run `atlas index` on the atlas repo itself and record wall-clock time. This serves as the regression baseline.
2. **Per-phase measurement:** After each phase, re-run `atlas index` on the same repo and compare to baseline. If overhead exceeds 5%, profile with `go test -bench` on the extraction functions to identify hot regexes.
3. **Benchmark tests:** Each extraction function gets a `Benchmark*` test in its test file using a representative fixture. This allows `go test -bench` to isolate per-function cost.
4. **Compiled regex enforcement:** All regexes are `var`-level `regexp.MustCompile` — the linter and code review catch any per-call compilation.

This is a local CLI tool with no production SLAs or alerting infrastructure. Performance is verified through benchmarks and manual measurement during development, not runtime metrics.

---

## Dependency Graph

```
Phase 1 (Comment Filter)
    │
    ├──► Phase 2 (Routes + Config + Tests)
    │        │
    │        ├──► Phase 3 (SQL + Services)
    │        │        │
    │        │        ├──► Phase 4 (Calls)
    │        │        │        │
    │        │        │        └──► Phase 5 (Integration)
```

All phases are strictly sequential. Each phase builds on the `Extract()` wiring from the previous phase.

---

## Summary

| Phase | New Files | Modified Files | Reference Kinds Added | Artifact Kinds Added |
|---|---|---|---|---|
| 1 | 2 | 0 | — | — |
| 2 | 24 | 3 | `registers_route`, `uses_config`, `tests` | `route`, `env_var`, `config_key` |
| 3 | 18 | 3 | `touches_table`, `migrates`, `invokes_external_api` | `sql_query`, `migration`, `external_service`, `background_job` |
| 4 | 9 | 3 | `calls` | — |
| 5 | 3 | 3 | — | — |
| **Total** | **56** | **12** | **7** | **7** |
