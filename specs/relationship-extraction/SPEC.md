# SPEC.md — Regex-Based Relationship Extraction for TS/Python/Rust

## 1. Product Overview

### 1.1 Name
Relationship Extraction Expansion

### 1.2 One-Sentence Purpose
Add regex-based relationship and artifact extraction to the TypeScript, Python, and Rust extractors, bringing them closer to parity with the Go extractor's call graph, route, config, SQL, test, and service detection capabilities.

### 1.3 Product Vision
The Go extractor uses AST parsing to extract nine relationship types and seven artifact types. The TypeScript, Python, and Rust extractors currently extract only import references and zero artifacts. This spec defines regex-based extraction patterns for each language that capture the most valuable relationships without introducing new dependencies (no tree-sitter, no CGo).

### 1.4 Core Value Proposition
After implementation, agents querying a TypeScript, Python, or Rust repository through Atlas will be able to:
- Discover HTTP route registrations and their handlers
- Find environment variable and config key usage
- Identify SQL queries and migration files
- Link test functions to the symbols they test
- Detect external service calls (HTTP clients, RPC)
- Detect background job patterns (async tasks, worker queues)
- Follow function/method call patterns (heuristic confidence)

### 1.5 Scope Boundary
This spec covers regex-based extraction only. AST-based extraction via tree-sitter or language toolchains is out of scope and would be covered by a separate spec. All new references use `heuristic` or `likely` confidence unless the pattern is unambiguous.

---

## 2. Goals and Non-Goals

### 2.1 Primary Goals
1. Add route, config, SQL, test-reference, external-service, background-job, and call extraction to TypeScript, Python, and Rust extractors.
2. Produce the same `ReferenceRecord` and `ArtifactRecord` types that the Go extractor produces, ensuring downstream queries (`who-calls`, `list routes`, `tests-for`) work across all languages.
3. Maintain extraction performance within the existing indexing time budget (regex adds negligible overhead vs current symbol extraction).
4. Achieve no false negatives on common framework idioms (Express, Flask, FastAPI, Django, Actix, Axum, Rocket).

### 2.2 Non-Goals
1. Achieving `exact` confidence on call extraction — regex cannot resolve scope or overloads.
2. Cross-file interface/trait implementation detection — requires type resolution beyond single-file parsing.
3. Supporting every possible framework — focus on the top 2-3 frameworks per language.
4. Replacing the Go extractor's AST-based approach — Go keeps its current implementation.

---

## 3. Architecture

### 3.1 File Structure Per Extractor

Each extractor package gains new files matching the Go extractor's layout:

```
internal/extractor/{lang}extractor/
  {lang}.go        # existing — updated to call new extractors
  calls.go         # NEW — function/method call detection
  routes.go        # NEW — HTTP route registration detection
  config.go        # NEW — environment/config access detection
  sql.go           # NEW — SQL query and migration detection
  tests.go         # NEW — test-to-symbol reference generation
  services.go      # NEW — external service and background job detection
```

### 3.2 Function Signatures

All new extraction functions follow a consistent pattern per extractor:

```go
// References-only extractors
func extractCalls(content string, lines []string, codeLines []bool, symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord
func extractTestReferences(symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord

// References + Artifacts extractors (codeLines from commentfilter.LineFilter)
func extractRoutes(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
func extractConfigAccess(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
func extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
func extractServices(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
```

### 3.3 Wiring Into Extract()

Each extractor's `Extract()` method appends results from the new functions after existing symbol and import extraction:

```go
// After existing symbol extraction...
refs, arts := extractRoutes(content, lines)
result.References = append(result.References, refs...)
result.Artifacts = append(result.Artifacts, arts...)
// ... repeat for each extraction function
```

### 3.4 Line Number Calculation

All regex-based extractors compute line numbers from match positions using:

```go
line := strings.Count(content[:matchIndex], "\n") + 1
```

This is consistent with how the existing non-Go extractors already compute line numbers.

---

## 4. Extraction Patterns — TypeScript/JavaScript

### 4.1 Route Registration

**Reference kind:** `registers_route`
**Artifact kind:** `route`

| Framework | Pattern | Example |
|---|---|---|
| Express/Koa/Hapi | `app.METHOD('/path'` or `router.METHOD('/path'` where METHOD is `get\|post\|put\|delete\|patch\|head\|options\|all\|use` | `app.get('/users', handler)` |
| NestJS decorators | `@Get('/path')\|@Post('/path')\|@Put\|@Delete\|@Patch\|@Head\|@Options\|@All` | `@Get('/users/:id')` |
| Next.js file routes | Export of `GET\|POST\|PUT\|DELETE\|PATCH` function in file under `app/` directory | `export async function GET(req)` |

**Regex definitions:**

```
expressRouteRe = (?m)(?:app|router|server)\.(get|post|put|delete|patch|head|options|all|use)\s*\(\s*['"`]([^'"`]+)['"`]
nestDecoratorRe = (?m)@(Get|Post|Put|Delete|Patch|Head|Options|All)\s*\(\s*['"`]([^'"`]*)['"`]\s*\)
nextRouteExportRe = (?m)^export\s+(?:async\s+)?function\s+(GET|POST|PUT|DELETE|PATCH)\s*\(
```

**Artifact DataJSON:** `{"method": "GET", "path": "/users", "handler": "<next_function_name_or_empty>"}`
**Confidence:** `exact` when path is a string literal; `heuristic` otherwise.

### 4.2 Config/Environment Access

**Reference kind:** `uses_config`
**Artifact kind:** `env_var` or `config_key`

| Pattern | Example |
|---|---|
| `process.env.KEY` | `process.env.DATABASE_URL` |
| `process.env['KEY']` or `process.env["KEY"]` | `process.env['API_KEY']` |
| `config.get('key')` or `config.get<T>('key')` | `config.get('database.host')` |
| `dotenv` parsed results: direct property access after `config()` | — |

**Regex definitions:**

```
processEnvDotRe  = (?m)process\.env\.([A-Z_][A-Z0-9_]*)
processEnvBrackRe = (?m)process\.env\[['"]([^'"]+)['"]\]
configGetRe      = (?m)config\.get(?:<[^>]+>)?\s*\(\s*['"]([^'"]+)['"]
```

**Artifact DataJSON:** `{"key": "DATABASE_URL", "source": "process.env"}`
**Confidence:** `exact`

### 4.3 SQL Artifacts

**Reference kind:** `touches_table` or `migrates`
**Artifact kind:** `sql_query` or `migration`

| Pattern | Detection |
|---|---|
| String literals containing SQL keywords | Same approach as Go extractor: scan for `CREATE TABLE\|ALTER TABLE\|INSERT\|SELECT\|UPDATE\|DELETE\|DROP` at start of trimmed string |
| Template literals with SQL | Match backtick strings containing SQL keywords |
| Migration file path | File path contains `migrations/` or `migrate/` |

**Regex definitions:**

```
sqlKeywordRe = (?i)^\s*(CREATE\s+TABLE|ALTER\s+TABLE|INSERT|SELECT|UPDATE|DELETE|DROP)
```

Applied to string literal contents extracted via:
```
stringLiteralRe  = (?s)['"]([^'"]{20,})['"]
templateLiteralRe = (?s)`([^`]{20,})`
```

**Artifact DataJSON:** `{"query": "<truncated_to_200_chars>", "type": "SELECT"}`
**Confidence:** `heuristic`

### 4.4 Test References

**Reference kind:** `tests`

| Pattern | Target Symbol |
|---|---|
| `describe('SymbolName'` | `moduleName.SymbolName` |
| `it('should ... SymbolName'` | Not extracted (too ambiguous) |
| `test('SymbolName ...'` | `moduleName.SymbolName` if first word matches a known symbol |
| Function named `test_symbolName` or `testSymbolName` | `moduleName.symbolName` |

**Logic:** After symbol extraction, iterate test symbols. For `describe` blocks, the first argument string is the test target. For named test functions, strip `test`/`Test` prefix and match against extracted symbols.

**Confidence:** `heuristic`

### 4.5 External Services and Background Jobs

**Reference kind:** `invokes_external_api`
**Artifact kind:** `external_service` or `background_job`

| Pattern | Type |
|---|---|
| `fetch('/url'` or `fetch(url` | `http_client` |
| `axios.get\|post\|put\|delete\|patch\|request('/url'` | `http_client` |
| `http.get\|post('/url'` (Node http module) | `http_client` |
| `new Worker(` | `background_job` (type: `worker`) |
| `setTimeout\|setInterval(` | Not extracted (too common, low signal) |
| `Bull\|BullMQ\|agenda` queue patterns | `background_job` (type: `queue`) |

**Regex definitions:**

```
fetchRe    = (?m)fetch\s*\(\s*['"`]([^'"`]+)['"`]
axiosRe    = (?m)axios\.(get|post|put|delete|patch|request)\s*\(\s*['"`]([^'"`]+)['"`]
httpReqRe  = (?m)(?:http|https)\.(?:get|post|request)\s*\(\s*['"`]([^'"`]+)['"`]
workerRe   = (?m)new\s+Worker\s*\(
queueRe    = (?m)(?:new\s+(?:Queue|Bull|Agenda)\s*\(|\.process\s*\()
```

**Confidence:** `heuristic`

### 4.6 Calls

**Reference kind:** `calls`

| Pattern | Example | Confidence |
|---|---|---|
| `identifier(` (not preceded by `function\|class\|if\|for\|while\|switch\|return\|import\|export\|new\|catch\|typeof\|instanceof\|case\|throw\|await\|yield\|from\|of`) | `processData(x)` | `heuristic` |
| `expr.method(` | `service.getData()` | `heuristic` |

**Regex definitions:**

```
directCallRe  = (?m)(?:^|[^.\w])([a-z]\w*)\s*\(
selectorCallRe = (?m)(\w+)\.(\w+)\s*\(
```

**Post-filter:** Exclude matches where the identifier is a language keyword. Exclude matches inside string literals or comments (heuristic: skip lines starting with `//` or `*`, and skip content between `/*` and `*/` — best-effort, not perfect).

**ReferenceRecord:**
- `RawTargetText`: `"identifier"` or `"receiver.method"`
- `Confidence`: `heuristic`

**Note:** Call extraction has the highest false-positive rate. It is intentionally last in priority. Downstream consumers should filter by confidence level.

---

## 5. Extraction Patterns — Python

### 5.1 Route Registration

**Reference kind:** `registers_route`
**Artifact kind:** `route`

| Framework | Pattern | Example |
|---|---|---|
| Flask | `@app.route('/path')` or `@app.METHOD('/path')` or `@blueprint.route(...)` | `@app.route('/users', methods=['GET'])` |
| FastAPI | `@app.get('/path')\|@router.get('/path')` where method is `get\|post\|put\|delete\|patch\|head\|options` | `@router.get('/users/{id}')` |
| Django urls | `path('url/', view)` or `re_path(r'pattern', view)` or `url(r'pattern', view)` | `path('users/', views.UserList.as_view())` |

**Regex definitions:**

```
flaskRouteRe   = (?m)@(?:\w+)\.(route|get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]
fastapiRouteRe = (?m)@(?:\w+)\.(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]
djangoPathRe   = (?m)(?:path|re_path|url)\s*\(\s*['"]([^'"]+)['"]
```

Note: `flaskRouteRe` and `fastapiRouteRe` share the same pattern. The `route` method maps to `ANY`.

**Artifact DataJSON:** `{"method": "GET", "path": "/users/{id}", "handler": ""}`
**Confidence:** `exact`

### 5.2 Config/Environment Access

**Reference kind:** `uses_config`
**Artifact kind:** `env_var` or `config_key`

| Pattern | Example |
|---|---|
| `os.environ['KEY']` or `os.environ["KEY"]` | `os.environ['DATABASE_URL']` |
| `os.environ.get('KEY')` | `os.environ.get('API_KEY', 'default')` |
| `os.getenv('KEY')` | `os.getenv('SECRET_KEY')` |
| `settings.KEY` (Django convention) | `settings.DEBUG` |
| `config['KEY']` or `config.get('KEY')` | `config['database']['host']` |

**Regex definitions:**

```
osEnvironBrackRe = (?m)os\.environ\[['"]([^'"]+)['"]\]
osEnvironGetRe   = (?m)os\.environ\.get\s*\(\s*['"]([^'"]+)['"]
osGetenvRe       = (?m)os\.getenv\s*\(\s*['"]([^'"]+)['"]
settingsDotRe    = (?m)settings\.([A-Z_][A-Z0-9_]*)
```

**Confidence:** `exact`

### 5.3 SQL Artifacts

Same approach as TypeScript (Section 4.3). Python additionally matches:
- Triple-quoted strings: `"""..."""` and `'''...'''`
- f-strings containing SQL: `f"SELECT ... {var}"`

```
tripleQuoteRe = (?s)(?:\"\"\"(.*?)\"\"\"|'''(.*?)''')
```

**Confidence:** `heuristic`

### 5.4 Test References

**Reference kind:** `tests`

| Pattern | Target Symbol |
|---|---|
| Function named `test_symbol_name` | `moduleName.symbol_name` |
| Method named `test_symbol_name` in `TestCase` subclass | `moduleName.symbol_name` |
| Class named `TestSymbolName` | `moduleName.SymbolName` |

**Logic:** For test functions (already detected by existing extractor), strip `test_` prefix. If the remainder matches an extracted symbol name (case-insensitive), generate a `tests` reference. For test classes, strip `Test` prefix and match similarly.

**Confidence:** `heuristic`

### 5.5 External Services and Background Jobs

**Reference kind:** `invokes_external_api`
**Artifact kind:** `external_service` or `background_job`

| Pattern | Type |
|---|---|
| `requests.get\|post\|put\|delete\|patch\|head\|options('/url'` | `http_client` |
| `httpx.get\|post\|put\|delete('/url'` or `httpx.AsyncClient` | `http_client` |
| `urllib.request.urlopen(` | `http_client` |
| `aiohttp.ClientSession` | `http_client` |
| `celery` task decorators: `@app.task`, `@shared_task` | `background_job` (type: `celery_task`) |
| `threading.Thread(target=` | `background_job` (type: `thread`) |
| `asyncio.create_task(` | `background_job` (type: `async_task`) |
| `subprocess.run\|Popen\|call(` | `background_job` (type: `subprocess`) |

**Regex definitions:**

```
requestsRe     = (?m)requests\.(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]
httpxRe        = (?m)httpx\.(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]
urllibRe       = (?m)urllib\.request\.urlopen\s*\(
celeryTaskRe   = (?m)@(?:app\.task|shared_task|celery\.task)
threadRe       = (?m)threading\.Thread\s*\(
asyncTaskRe    = (?m)asyncio\.create_task\s*\(
subprocessRe   = (?m)subprocess\.(?:run|Popen|call)\s*\(
```

**Confidence:** `heuristic`

### 5.6 Calls

**Reference kind:** `calls`

Same approach as TypeScript (Section 4.6) adapted for Python syntax:

| Pattern | Example | Confidence |
|---|---|---|
| `identifier(` (not preceded by `def\|class\|if\|elif\|for\|while\|with\|except\|import\|from\|as\|and\|or\|not\|in\|is\|lambda\|assert\|raise\|return\|yield\|del\|global\|nonlocal`) | `process_data(x)` | `heuristic` |
| `expr.method(` | `service.get_data()` | `heuristic` |

**Regex definitions:**

```
directCallRe   = (?m)(?:^|[^.\w])([a-z_]\w*)\s*\(
selectorCallRe = (?m)(\w+)\.(\w+)\s*\(
```

**Post-filter:** Exclude Python keywords and builtins (`print`, `len`, `range`, `str`, `int`, `list`, `dict`, `set`, `tuple`, `type`, `super`, `isinstance`, `issubclass`, `hasattr`, `getattr`, `setattr`).

**Confidence:** `heuristic`

---

## 6. Extraction Patterns — Rust

### 6.1 Route Registration

**Reference kind:** `registers_route`
**Artifact kind:** `route`

| Framework | Pattern | Example |
|---|---|---|
| Actix-web | `#[get("/path")]\|#[post("/path")]` etc. | `#[get("/users/{id}")]` |
| Actix-web resource | `.route("/path", web::get().to(handler))` | `.route("/users", web::get().to(list_users))` |
| Rocket | `#[get("/path")]\|#[post("/path")]` etc. | `#[post("/login", data = "<input>")]` |
| Axum | `.route("/path", get(handler))` | `.route("/users", get(list_users).post(create_user))` |

**Regex definitions:**

```
attrRouteRe  = (?m)#\[(get|post|put|delete|patch|head|options)\s*\(\s*"([^"]+)"
axumRouteRe  = (?m)\.route\s*\(\s*"([^"]+)"\s*,\s*(get|post|put|delete|patch|head|options)\s*\(
actixRouteRe = (?m)\.route\s*\(\s*"([^"]+)"\s*,\s*web::(get|post|put|delete|patch|head|options)\s*\(\s*\)\s*\.to\s*\(\s*(\w+)
```

**Confidence:** `exact` for attribute macros; `heuristic` for builder patterns.

### 6.2 Config/Environment Access

**Reference kind:** `uses_config`
**Artifact kind:** `env_var` or `config_key`

| Pattern | Example |
|---|---|
| `std::env::var("KEY")` or `env::var("KEY")` | `env::var("DATABASE_URL")` |
| `std::env::var_os("KEY")` | `env::var_os("HOME")` |
| `dotenv::var("KEY")` or `dotenvy::var("KEY")` | `dotenv::var("API_KEY")` |
| `config.get::<T>("key")` or `config.get_string("key")` | `config.get_string("database.url")` |

**Regex definitions:**

```
envVarRe    = (?m)(?:std::)?env::var(?:_os)?\s*\(\s*"([^"]+)"
dotenvVarRe = (?m)(?:dotenv|dotenvy)::var\s*\(\s*"([^"]+)"
configGetRe = (?m)config\.get(?:_string|_int|_float|_bool|_array|_table)?(?:::<[^>]+>)?\s*\(\s*"([^"]+)"
```

**Confidence:** `exact`

### 6.3 SQL Artifacts

Same approach as TypeScript (Section 4.3). Rust additionally matches:
- Raw string literals: `r"..."` and `r#"..."#`
- `sqlx::query!("...")` and `sqlx::query_as!("...")`
- `diesel` DSL patterns (heuristic — `.filter()`, `.select()` chains are too generic to extract reliably, so only string-literal SQL is captured)

```
rawStringRe  = (?s)r#*"(.*?)"#*
sqlxQueryRe  = (?m)(?:sqlx::)?query(?:_as)?!?\s*\(\s*r?#*"([^"]+)"
```

**Confidence:** `heuristic`

### 6.4 Test References

**Reference kind:** `tests`

| Pattern | Target Symbol |
|---|---|
| Function named `test_symbol_name` with `#[test]` attribute | `moduleName::symbol_name` |
| Function named `test_symbol_name` with `#[tokio::test]` | `moduleName::symbol_name` |

**Logic:** The existing Rust extractor already detects test functions. Strip `test_` prefix from test function names and match against extracted symbol names.

**Confidence:** `heuristic`

### 6.5 External Services and Background Jobs

**Reference kind:** `invokes_external_api`
**Artifact kind:** `external_service` or `background_job`

| Pattern | Type |
|---|---|
| `reqwest::get\|Client::new` | `http_client` |
| `hyper::Client` | `http_client` |
| `tonic` channel/client patterns | `grpc_client` |
| `tokio::spawn(` | `background_job` (type: `async_task`) |
| `std::thread::spawn(` or `thread::spawn(` | `background_job` (type: `thread`) |
| `rayon` parallel iterators | `background_job` (type: `parallel`) |

**Regex definitions:**

```
reqwestRe      = (?m)reqwest::(?:get|Client)
hyperClientRe  = (?m)hyper::Client
tonicRe        = (?m)tonic::\w+
tokioSpawnRe   = (?m)tokio::spawn\s*\(
threadSpawnRe  = (?m)(?:std::)?thread::spawn\s*\(
rayonRe        = (?m)\.par_iter\s*\(
```

**Confidence:** `heuristic`

### 6.6 Calls

**Reference kind:** `calls`

| Pattern | Example | Confidence |
|---|---|---|
| `identifier(` (not preceded by `fn\|let\|mut\|const\|static\|struct\|enum\|trait\|type\|impl\|mod\|use\|pub\|if\|else\|for\|while\|loop\|match\|return\|where\|as\|in\|ref\|move`) | `process_data(x)` | `heuristic` |
| `path::func(` | `service::get_data()` | `heuristic` |
| `expr.method(` | `client.send()` | `heuristic` |
| `macro!(` | `println!("...")` | `heuristic` |

**Regex definitions:**

```
directCallRe   = (?m)(?:^|[^:.\w])([a-z_]\w*)\s*\(
pathCallRe     = (?m)(\w+(?:::\w+)+)\s*\(
selectorCallRe = (?m)(\w+)\.(\w+)\s*\(
macroCallRe    = (?m)(\w+)!\s*[\(\[\{]
```

**Post-filter:** Exclude Rust keywords. Exclude common macros that are not meaningful calls (`println`, `eprintln`, `dbg`, `format`, `vec`, `assert`, `assert_eq`, `assert_ne`, `cfg`, `derive`, `todo`, `unimplemented`, `unreachable`, `panic`).

**Confidence:** `heuristic`

---

## 7. Comment and String Filtering

### 7.1 Problem

Regex-based extraction can match patterns inside comments and string literals, producing false positives. A best-effort filter reduces noise without requiring a full parser.

### 7.2 Approach

Before running extraction regexes, apply a **line-level pre-filter** that marks lines as code or non-code:

| Language | Single-line comment | Block comment start/end |
|---|---|---|
| TypeScript/JS | `//` | `/*` ... `*/` |
| Python | `#` | `"""` ... `"""` and `'''` ... `'''` (also used as strings — accept some ambiguity) |
| Rust | `//` | `/*` ... `*/` (nestable) |

**Implementation:** Build a `[]bool` (one entry per line) indicating whether each line is inside a block comment. Extraction functions skip matches on comment lines. This is imperfect (e.g., a comment start inside a string literal) but eliminates the vast majority of false positives.

### 7.3 Shared Utility

Implement a shared `commentfilter` package in `internal/extractor/commentfilter/` with:

```go
// LineFilter marks each line as code (true) or comment (false).
func LineFilter(content string, lang string) []bool
```

Supported `lang` values: `"typescript"`, `"python"`, `"rust"`.

---

## 8. Output Compatibility

### 8.1 Reference Kinds

All new extraction produces `ReferenceRecord` values with the same `ReferenceKind` strings used by the Go extractor:

| Kind | Produced By |
|---|---|
| `calls` | Call extraction (all languages) |
| `registers_route` | Route extraction (all languages) |
| `uses_config` | Config extraction (all languages) |
| `touches_table` | SQL extraction (all languages) |
| `migrates` | SQL extraction — migration files (all languages) |
| `invokes_external_api` | Service extraction (all languages) |
| `tests` | Test reference extraction (all languages) |

### 8.2 Artifact Kinds

| Kind | Produced By |
|---|---|
| `route` | Route extraction |
| `env_var` | Config extraction (env variables) |
| `config_key` | Config extraction (config files/objects) |
| `sql_query` | SQL extraction |
| `migration` | SQL extraction — migration files |
| `external_service` | Service extraction (HTTP/RPC clients) |
| `background_job` | Service extraction (async tasks, workers, threads) |

### 8.3 Confidence Values

| Value | Meaning | When Used |
|---|---|---|
| `exact` | Pattern is unambiguous | String-literal route paths, env var names, config keys |
| `likely` | High probability but not proven | — (reserved; regex extraction rarely achieves this) |
| `heuristic` | Pattern-matched, may have false positives | Calls, SQL detection, service detection, test references |

---

## 9. Testing Strategy

### 9.1 Unit Tests Per Extraction Function

Each new file (e.g., `tsextractor/routes.go`) has a corresponding test file (`tsextractor/routes_test.go`) with:
- **Positive cases:** Common framework patterns produce expected references/artifacts.
- **Negative cases:** Similar-looking non-matches (e.g., `app.get` as a variable access, not a route) do not produce false positives.
- **Comment filtering:** Patterns inside comments are not extracted.
- **Edge cases:** Multi-line strings, template literals, nested calls.

### 9.2 Integration Tests

The existing extractor integration tests (which call `Extract()` and verify the full result) are extended to include test fixtures containing route registrations, config access, SQL queries, and call patterns for each language.

### 9.3 Test Fixture Files

Each extractor gains test fixture files in the test directory:

```
internal/extractor/{lang}extractor/testdata/
  routes.{ext}       # file with route patterns
  config.{ext}       # file with config access patterns
  sql.{ext}          # file with SQL patterns
  services.{ext}     # file with service/job patterns
  calls.{ext}        # file with various call patterns
```

---

## 10. Performance

### 10.1 Constraints

- Each new extraction function adds one or more compiled regex passes over the file content.
- All regexes must be compiled once at package init time (`var re = regexp.MustCompile(...)`), not per invocation.
- The comment line filter is computed once per `Extract()` call and shared across all extraction functions.

### 10.2 Expected Impact

- Regex extraction adds < 1ms per file for typical source files (< 5000 lines).
- No impact on memory beyond the `[]bool` comment filter and the returned records.
- Total indexing time increase for a repository should be < 5% over current extraction.

---

## 11. Implementation Priority

### Phase 1: Routes + Config + Tests (highest value, lowest risk)
- Route patterns are distinctive and framework-specific — low false-positive rate.
- Config/env patterns are simple string matches — very reliable.
- Test references reuse existing symbol data — minimal new regex needed.
- Comment filter utility (shared dependency for all phases).

### Phase 2: SQL + Services
- SQL detection ports directly from Go extractor logic.
- Service patterns are framework-specific with moderate false-positive risk.

### Phase 3: Calls (highest value, highest noise)
- Call extraction has inherent limitations with regex.
- Ship with `heuristic` confidence and iterate based on real-world false-positive rates.

---

## 12. Open Questions

1. **Should call extraction be gated by a config flag?** Given the heuristic nature, users may want to disable it for noisy codebases. Recommendation: no flag initially; confidence filtering at query time is sufficient.
2. **Should the comment filter handle string-literal false positives?** A line containing `"// not a comment"` would be misclassified. Recommendation: accept this as a known limitation; the false-negative rate is negligible.
3. **Should framework detection be explicit (config) or implicit (pattern matching)?** Recommendation: implicit. If the patterns match, extract. No framework declaration needed.
