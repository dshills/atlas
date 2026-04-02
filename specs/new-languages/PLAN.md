# PLAN.md — Java, C#, Swift, and Lua Language Extractors

## Overview

This plan implements four new language extractors following the established extractor pattern used by the TypeScript, Python, and Rust extractors. Each phase is independently implementable and testable.

**Spec reference:** `specs/new-languages/SPEC.md`

---

## Phase 1: Comment Filter Updates and Java Structural Extractor

### Goals
1. Add new language aliases to `commentfilter.LineFilter`: `"java"`, `"csharp"` (C-style non-nestable), `"swift"` (C-style nestable), `"lua"` (new `--` / `--[[ ]]` mode).
2. Implement the Java extractor with structural extraction only: imports, symbols, qualified names, visibility, block-end detection, and test detection.
3. Register the Java extractor in the registry and update config defaults.

### Files to Create
- `internal/extractor/javaextractor/java.go` — `JavaExtractor` struct implementing `Extractor` interface; `Language()` returns `"java"`, `Supports()` checks `.java`; `Extract()` wires imports + symbols only (relationship extraction added in Phase 3). Contains `extractImports()`, `extractSymbols()`, `findBlockEnd()`, `visibility()`, and all regex patterns from spec sections 3.4–3.9.
- `internal/extractor/javaextractor/java_test.go` — Unit tests for symbol extraction (classes, interfaces, enums, methods, fields, constants, annotations), import extraction, qualified name generation, visibility, block-end detection, and test detection (both JUnit annotation and TestCase strategies). Include `TestExtract_FullPipeline` integration test.

### Files to Modify
- `internal/extractor/commentfilter/commentfilter.go` — Add `"java"`, `"csharp"` as aliases in the switch statement routing to `filterCStyle(lines, result, false)`. Add `"swift"` routing to `filterCStyle(lines, result, true)`. Add `"lua"` case to the switch statement, routing to a new internal helper `filterLua(lines []string, result []bool)` (same signature as the existing `filterCStyle` and `filterPython` helpers — these are internal functions called by `LineFilter`, not part of the public API). This helper handles `--` single-line and `--[[ ]]` block comments.
- `internal/extractor/commentfilter/commentfilter_test.go` — Add tests for all four new language identifiers: `"java"` and `"csharp"` behave like `"typescript"`, `"swift"` behaves like `"rust"` (nested `/* */`), `"lua"` handles `--` single-line and `--[[ ]]` block comments.
- `internal/config/config.go` — Add `Java bool` field to `LanguageConfig` struct with `yaml:"java" json:"java"` tags.
- `internal/config/defaults.go` — Add `"**/*.java"` to `Include`, `"build/**"` to `Exclude`, `Java: true` to `Languages` in `DefaultConfig()`. Add matching entries in `DefaultConfigYAML()`.
- `internal/config/config_test.go` — Update `TestDefaultConfig` to verify the new Java language flag and include/exclude entries.
- `cmd/atlas/main.go` — Add `import "github.com/dshills/atlas/internal/extractor/javaextractor"` and `reg.Register(javaextractor.New())`.

### Key Decisions
- Java is implemented first because it has the most complex symbol extraction (generics in method signatures, annotation types, two test detection strategies) and serves as the template for the remaining three extractors.
- The `filterLua` comment filter is implemented in Phase 1 alongside the other aliases so all comment filtering is complete before any extractor phases need it.
- Relationship extraction (routes, config, SQL, services, calls, test references) is deferred to Phase 3 to keep this phase focused and testable.

### Risks
- Java method regex may false-match field declarations with initializers that contain parentheses (e.g., `Foo bar = new Foo()`). Mitigate by requiring the method regex to match indented lines with a return type followed by an identifier and `(`.
- `TestCase`-style test detection requires a two-pass approach (check for `extends TestCase`, then scan methods). This is manageable within a single `extractSymbols` function.

### Acceptance Criteria
- `go test ./internal/extractor/javaextractor/...` passes.
- `go test ./internal/extractor/commentfilter/...` passes with new language tests.
- `go test ./internal/config/...` passes with updated defaults.
- `golangci-lint run ./...` passes.
- Java extractor correctly extracts: package declarations, classes (public/private/abstract/final/static), interfaces, enums, methods (including generics), static final constants, annotation types, JUnit `@Test`/`@ParameterizedTest` tests, and TestCase-style `test*` methods.
- Comment filter correctly handles all four new language identifiers.
- `TestExtract_FullPipeline` passes for the Java extractor (structural extraction only at this phase).

---

## Phase 2: C#, Swift, and Lua Structural Extractors

### Goals
1. Implement C# extractor with structural extraction: imports (`using`), symbols (namespace, class, interface, enum, struct, record, method, property, const, test detection via `[Test]`/`[Fact]`/`[Theory]`/`[TestMethod]`).
2. Implement Swift extractor with structural extraction: imports (`import`), symbols (class, struct, enum, protocol, extension, function, method, property, const, test detection via `test*` prefix and `@Test`).
3. Implement Lua extractor with structural extraction: imports (`require`), symbols (function declarations, function assignments, methods via `:`, local vars, module vars, test detection via `test*` prefix and busted `describe`/`it`). Lua-specific `findBlockEnd` using `end` keyword depth counting.
4. Register all three extractors and update config defaults.

### Files to Create
- `internal/extractor/csharpextractor/csharp.go` — `CSharpExtractor` struct; `Language()` returns `"csharp"`, `Supports()` checks `.cs`; symbol regexes from spec section 4.5; `extractImports()` for `using` statements; test detection per spec section 4.8; visibility rules per spec section 4.7; brace-counting `findBlockEnd()`.
- `internal/extractor/csharpextractor/csharp_test.go` — Unit tests for all C# symbol kinds, imports, qualified names, visibility, test detection, block-end. `TestExtract_FullPipeline`.
- `internal/extractor/swiftextractor/swift.go` — `SwiftExtractor` struct; `Language()` returns `"swift"`, `Supports()` checks `.swift`; symbol regexes from spec section 5.5; `extractImports()` for `import` statements; test detection per spec section 5.8; visibility rules per spec section 5.7; brace-counting `findBlockEnd()`.
- `internal/extractor/swiftextractor/swift_test.go` — Unit tests for all Swift symbol kinds, imports, qualified names, visibility, test detection, block-end. `TestExtract_FullPipeline`.
- `internal/extractor/luaextractor/lua.go` — `LuaExtractor` struct; `Language()` returns `"lua"`, `Supports()` checks `.lua`; symbol regexes from spec section 6.5; `extractImports()` for `require`; test detection per spec section 6.9; Lua-specific `findBlockEnd()` using keyword-based depth counting per spec section 6.8; visibility: `local` → unexported, else exported.
- `internal/extractor/luaextractor/lua_test.go` — Unit tests for Lua symbols (function declarations, assignments, methods, vars), imports, qualified names, visibility, test detection, Lua block-end detection. `TestExtract_FullPipeline`.

### Files to Modify
- `internal/config/config.go` — Add `CSharp bool`, `Swift bool`, `Lua bool` fields to `LanguageConfig`.
- `internal/config/defaults.go` — Add `"**/*.cs"`, `"**/*.swift"`, `"**/*.lua"` to `Include`; `"bin/**"`, `"obj/**"`, `".build/**"`, `"Packages/**"` to `Exclude`; `CSharp: true`, `Swift: true`, `Lua: true` to `Languages`. Update `DefaultConfigYAML()` to match.
- `internal/config/config_test.go` — Update `TestDefaultConfig` to verify three new language flags and new include/exclude entries.
- `cmd/atlas/main.go` — Add imports for `csharpextractor`, `swiftextractor`, `luaextractor` and register all three.

### Key Decisions
- All three extractors are implemented together because they share the same structure and can be developed/tested in parallel. Each is independent — no cross-language dependencies.
- Lua's `findBlockEnd` is the only non-brace-counting implementation. It uses keyword-based depth tracking per spec section 6.8: `function`, `if`, `for`, `while`, `repeat` increment depth; bare `do` at line start increments; `end` decrements; `until` decrements for `repeat` blocks.
- Swift module name is derived from the file name (sans extension) since Swift doesn't have an explicit module declaration at the file level.
- C# module name is derived from the `namespace` declaration.

### Risks
- Lua's keyword-based block detection may miscount on lines with multiple block-opening keywords. Mitigation: per spec, this is a known limitation consistent with other regex-based extractors.
- C# `partial` classes split across files could cause incomplete symbol extraction. Mitigation: each file is processed independently; partial class in each file gets its own symbol entry. This is the same approach as Go interface extraction across files.

### Acceptance Criteria
- `go test ./internal/extractor/csharpextractor/...` passes.
- `go test ./internal/extractor/swiftextractor/...` passes.
- `go test ./internal/extractor/luaextractor/...` passes.
- `go test ./internal/config/...` passes.
- `golangci-lint run ./...` passes.
- Each extractor correctly produces symbols, imports, qualified names, visibility, and test detection per its spec section.
- Lua block-end detection correctly handles nested `function`/`if`/`for`/`while`/`repeat` with `end` closers.
- `TestExtract_FullPipeline` passes for each of the three extractors (structural extraction only at this phase).

---

## Phase 3: Java Relationship Extraction

### Goals
Add the full relationship extraction pipeline to the Java extractor: routes (Spring MVC, JAX-RS), config (System.getenv, System.getProperty, @Value), SQL (string literals with SQL keywords, JDBC execute methods, JPA @Query), services (HttpClient, RestTemplate, WebClient, OkHttpClient for external services; ExecutorService, CompletableFuture, @Async, Thread for background jobs), calls (directCallRe/selectorCallRe with Java keyword exclusion), and test references.

### Files to Create
- `internal/extractor/javaextractor/routes.go` — `extractRoutes()` function with `springMappingRe`, `jaxrsPathRe`, `jaxrsMethodRe` patterns. Produces `registers_route` references and `route` artifacts with `DataJSON` containing `{"method": "...", "path": "...", "handler": "..."}`.
- `internal/extractor/javaextractor/routes_test.go` — Tests for Spring MVC `@GetMapping`, `@PostMapping`, `@RequestMapping` patterns; JAX-RS `@Path` + `@GET`/`@POST` combinations. Tests for line numbers, comment filtering, and artifact DataJSON structure.
- `internal/extractor/javaextractor/config.go` — `extractConfigAccess()` function with `systemGetenvRe`, `systemPropertyRe`, `springValueRe` patterns. Produces `uses_config` references and `env_var`/`config_key` artifacts.
- `internal/extractor/javaextractor/config_test.go` — Tests for `System.getenv("KEY")`, `System.getProperty("key")`, `@Value("${key}")` patterns. Comment filtering tests.
- `internal/extractor/javaextractor/sql.go` — `extractSQLArtifacts()` function detecting SQL keywords in string literals and JDBC `execute*` methods, JPA `@Query` annotations. Produces `touches_table`/`migrates` references and `sql_query`/`migration` artifacts. Deduplicates by start byte offset.
- `internal/extractor/javaextractor/sql_test.go` — Tests for SQL keyword detection in string literals, JDBC Statement.executeQuery/executeUpdate patterns, JPA @Query annotation.
- `internal/extractor/javaextractor/services.go` — `extractServices()` function detecting HTTP clients (`HttpClient`, `RestTemplate`, `WebClient`, `OkHttpClient`) and background job patterns (`ExecutorService`, `CompletableFuture`, `@Async`, `new Thread`). Produces `invokes_external_api` references and `external_service`/`background_job` artifacts.
- `internal/extractor/javaextractor/services_test.go` — Tests for HTTP client and background job pattern detection.
- `internal/extractor/javaextractor/calls.go` — `extractCalls()` function using shared `directCallRe`/`selectorCallRe` patterns with Java keyword exclusion set. Deduplicates by `"line:kind:name"` key per spec section 2.3.
- `internal/extractor/javaextractor/calls_test.go` — Tests for direct calls, selector calls, keyword exclusion, comment filtering, deduplication.
- `internal/extractor/javaextractor/tests.go` — `extractTestReferences()` function matching test symbols to target symbols by stripping `test`/`Test` prefix and comparing case-insensitively. Produces `tests` references.
- `internal/extractor/javaextractor/tests_test.go` — Tests for test-to-symbol reference matching.

### Files to Modify
- `internal/extractor/javaextractor/java.go` — Wire all new extraction functions into `Extract()` method after symbol extraction: call `commentfilter.LineFilter(content, "java")`, then invoke `extractRoutes`, `extractConfigAccess`, `extractSQLArtifacts`, `extractServices`, `extractCalls`, `extractTestReferences` and append results to `result.References` and `result.Artifacts`.

### Key Decisions
- Java call extraction uses the same `directCallRe`/`selectorCallRe` patterns as Python and TypeScript, with a Java-specific keyword exclusion set of 46 reserved keywords.
- JAX-RS route extraction requires combining `@Path` and `@GET`/`@POST` annotations that may be on different lines. Strategy: collect all `@Path` annotations with their line numbers, then for each `@GET`/`@POST`/`@PUT`/`@DELETE`/`@PATCH`/`@HEAD`/`@OPTIONS`, find the nearest preceding `@Path` to construct the full route.
- `encoding/json.Marshal` is used for `DataJSON` fields, not `fmt.Sprintf`.

### Risks
- JAX-RS class-level `@Path` combined with method-level `@Path` requires multi-line correlation. Mitigate by scanning backwards from each method annotation to find the nearest `@Path`.
- Java string concatenation in SQL (e.g., `"SELECT * FROM " + table`) won't be fully captured. This is a known limitation of regex-based extraction — the first string fragment containing SQL keywords will still be detected.

### Acceptance Criteria
- `go test ./internal/extractor/javaextractor/...` passes (all test files).
- `golangci-lint run ./...` passes.
- Java extractor correctly produces route artifacts for Spring MVC and JAX-RS patterns.
- Config extraction detects `System.getenv`, `System.getProperty`, and `@Value` patterns.
- SQL extraction detects keywords in string literals, JDBC execute methods, and JPA @Query.
- Service detection identifies HTTP clients and background job patterns.
- Call extraction produces deduplicated call references with keyword exclusion.
- Test references link test symbols to their target symbols.
- `TestExtract_FullPipeline` covers the complete extraction pipeline.

---

## Phase 4: C#, Swift, and Lua Relationship Extraction

### Goals
Add the full relationship extraction pipeline to C#, Swift, and Lua extractors (routes, config, SQL, services, calls, test references).

### Files to Create

**C# (`internal/extractor/csharpextractor/`)**
- `routes.go` + `routes_test.go` — ASP.NET attributes (`[HttpGet]`, `[Route]`) and Minimal API (`app.MapGet`) patterns per spec section 4.9.
- `config.go` + `config_test.go` — `Environment.GetEnvironmentVariable`, `configuration["key"]`, `GetSection`, `GetValue` per spec section 4.10.
- `sql.go` + `sql_test.go` — SQL keywords in string/interpolated literals, EF Core `FromSqlRaw`. Deduplication by byte offset.
- `services.go` + `services_test.go` — `HttpClient`/`WebRequest`/`RestClient` for external services; `Task.Run`/`Thread`/`BackgroundService`/`Parallel.ForEach` for background jobs.
- `calls.go` + `calls_test.go` — Shared call patterns with C# keyword exclusion set (52 reserved keywords per spec).
- `tests.go` + `tests_test.go` — Test-to-symbol reference matching.

**Swift (`internal/extractor/swiftextractor/`)**
- `routes.go` + `routes_test.go` — Vapor route patterns (`app.get`, `app.post`) per spec section 5.9.
- `config.go` + `config_test.go` — `ProcessInfo.processInfo.environment["key"]`, `Environment.get("key")` per spec section 5.10.
- `sql.go` + `sql_test.go` — SQL keywords in string literals, Core Data `NSPredicate(format:)`, GRDB `.filter(sql:)`.
- `services.go` + `services_test.go` — `URLSession`/`Alamofire` for external services; `DispatchQueue`/`Task {`/`async let` for background jobs.
- `calls.go` + `calls_test.go` — Shared call patterns with Swift keyword exclusion set (43 keywords per spec).
- `tests.go` + `tests_test.go` — Test-to-symbol reference matching.

**Lua (`internal/extractor/luaextractor/`)**
- `routes.go` + `routes_test.go` — Lapis/OpenResty route patterns per spec section 6.10. Supports `get`, `post`, `put`, `delete` HTTP methods via `lapisRouteRe`.
- `config.go` + `config_test.go` — `os.getenv("key")` per spec section 6.11.
- `sql.go` + `sql_test.go` — SQL keywords on code lines per spec section 6.13.
- `services.go` + `services_test.go` — `http.request`/`socket.http` for external services; `ngx.timer.at`/`copas.addthread` for background jobs.
- `calls.go` + `calls_test.go` — Shared call patterns with Lua keyword exclusion set (22 keywords per spec). Lua also uses colon-call syntax (`obj:method()`); add a `colonCallRe` pattern: `(\w+):(\w+)\s*\(`.
- `tests.go` + `tests_test.go` — Test-to-symbol reference matching.

### Files to Modify
- `internal/extractor/csharpextractor/csharp.go` — Wire all extraction functions into `Extract()` after symbol extraction, using `commentfilter.LineFilter(content, "csharp")`.
- `internal/extractor/swiftextractor/swift.go` — Wire all extraction functions into `Extract()` after symbol extraction, using `commentfilter.LineFilter(content, "swift")`.
- `internal/extractor/luaextractor/lua.go` — Wire all extraction functions into `Extract()` after symbol extraction, using `commentfilter.LineFilter(content, "lua")`.

### Key Decisions
- All three languages are done together because Phase 3 established the Java relationship extraction pattern; the remaining three languages follow the same structure with language-specific regex patterns.
- Lua adds a `colonCallRe` pattern for Lua's method call syntax (`obj:method()`) in addition to the shared `directCallRe`/`selectorCallRe`.
- C# interpolated strings (`$"..."`) are treated as regular strings for SQL detection — the interpolation expressions are scanned along with the rest of the string content. This may produce rare false positives, which is acceptable at `heuristic` confidence.

### Risks
- Phase creates 36 new files (12 per extractor). Mitigate by following the exact pattern established in Phase 3 (Java) and the existing TypeScript/Python/Rust extractors.
- Lua's limited framework ecosystem means route/service tests will be sparse. This is expected per spec non-goal 4.

### Acceptance Criteria
- `go test ./internal/extractor/csharpextractor/...` passes.
- `go test ./internal/extractor/swiftextractor/...` passes.
- `go test ./internal/extractor/luaextractor/...` passes.
- `golangci-lint run ./...` passes.
- Each extractor's `TestExtract_FullPipeline` covers the full pipeline (imports, symbols, routes, config, SQL, services, calls, test references).
- All route artifacts include correct `DataJSON` with method, path, and handler fields.
- Call extraction uses proper keyword exclusion for each language.
- Lua colon-call syntax (`obj:method()`) is correctly extracted.

---

## Phase 5: README Update and Final Validation

### Goals
1. Update `README.md` with new languages in the Language Support table and Framework detection table.
2. Run full test suite and linter as final validation.
3. Verify all four extractors work end-to-end with `atlas index` on sample files.

### Files to Modify
- `README.md` — Add rows for C#, Java, Lua, Swift to the Language Support table (alphabetical order). Add rows for C#, Java, Lua, Swift to the Framework detection table (alphabetical order). Update the configuration example to show new file extensions and language flags. Specific table content per spec section 10.

### Key Decisions
- Tables are kept in alphabetical order per established convention.
- Configuration example in README is updated to reflect new defaults.

### Risks
- Minimal risk — documentation-only changes.

### Acceptance Criteria
- `go test ./...` passes (full suite).
- `golangci-lint run ./...` passes.
- README Language Support table contains all 9 languages (C#, Go, Java, JavaScript, Lua, Python, Rust, Swift, TypeScript) in alphabetical order.
- README Framework detection table contains all 8 entries (C#, Go is not listed since it uses AST, Java, JavaScript/TS, Lua, Python, Rust, Swift) in alphabetical order.
- `go build ./cmd/atlas` succeeds.

---

## Phase Dependency Summary

```
Phase 1 (Comment filter + Java structural)
    ↓
Phase 2 (C#, Swift, Lua structural)
    ↓
Phase 3 (Java relationship extraction)
    ↓
Phase 4 (C#, Swift, Lua relationship extraction)
    ↓
Phase 5 (README + final validation)
```

Phases 2 and 3 have no mutual dependency — they could theoretically run in parallel since Phase 2 creates new extractors (C#/Swift/Lua structural) while Phase 3 adds relationship extraction to Java. However, implementing Java relationships first (Phase 3) establishes the pattern that Phase 4 follows for the remaining three languages, so the sequential order is preferred.

---

## Performance Verification

After Phase 5, verify that:
- `atlas index` on a mixed-language repository completes without errors.
- Incremental indexing correctly skips unchanged files for all new languages.
- No regressions in existing Go/TypeScript/Python/Rust extraction.

## Observability and Error Handling

All new extractors follow the same observability pattern as existing extractors:
- **Diagnostics**: Extraction errors and warnings are reported via `ExtractResult.File.ParseStatus` (set to `"partial"` on non-fatal errors, `"error"` on failures). These are persisted to the `diagnostics` table and visible via `atlas list diagnostics`.
- **Partial results**: Parse failures in one extraction step (e.g., route extraction) do not abort the entire file extraction. Each step is wrapped in error handling that logs a diagnostic and continues with the remaining steps.
- **Metrics**: The existing `index_runs` table tracks per-run statistics (files processed, duration). No new metrics infrastructure is needed.
- **Malformed input**: Files that fail regex extraction produce partial results with diagnostics. This is consistent with the spec's requirement that "extractors must return partial results with diagnostics on failure."
