# SPEC.md — Java, C#, Swift, and Lua Language Extractors

## 1. Product Overview

### 1.1 Name
New Language Extractors

### 1.2 One-Sentence Purpose
Add regex-based extractors for Java, C#, Swift, and Lua. Each extractor implements the `Extractor` interface and produces symbols, imports, calls, routes, config, SQL, services, and test references using the same `ReferenceRecord`/`ArtifactRecord`/`SymbolRecord` types and extraction pipeline as the existing TypeScript, Python, and Rust extractors.

### 1.3 Scope
Each new extractor follows the established pattern: symbol extraction, import detection, and the full relationship extraction pipeline (routes, config, SQL, services, calls, test references) using the shared comment filter. All extractors are regex/heuristic-based with no new dependencies.

---

## 2. Goals and Non-Goals

### 2.1 Goals
1. Add four new language extractors (Java, C#, Swift, Lua) registered in the extractor registry.
2. Each extractor supports the full extraction pipeline: imports, symbols, routes, config, SQL, services, calls, and test references.
3. Use identical reference kinds, artifact kinds, confidence levels, and extraction helper function signatures as existing extractors (see section 2.3).
4. Update configuration defaults to include new file extensions.

### 2.2 Non-Goals
1. Full semantic analysis (type resolution, overload resolution, generics).
2. Build system integration (Maven, NuGet, SPM, LuaRocks).
3. Supporting every framework — only 1-3 of the most popular web frameworks per language are covered (see per-language route extraction sections for specific framework lists).
4. Lua route extraction is limited to OpenResty/Lapis only (section 6.10). Lua service extraction is limited to `http.request`/`socket.http` and `ngx.timer.at`/`copas.addthread` (section 6.13).

### 2.3 Consistency with Existing Extractors

All new extractors implement the `Extractor` interface defined in `internal/extractor/extractor.go`:

```go
type Extractor interface {
    Language() string                                                    // e.g., "java"
    Supports(path string) bool                                           // file extension check
    SupportedKinds() []string                                            // symbol kinds this extractor produces
    Extract(ctx context.Context, req ExtractRequest) (*ExtractResult, error)
}

type ExtractRequest struct {
    FilePath string
    Content  []byte
    RepoRoot string
}

type ExtractResult struct {
    File       *FileRecord
    Package    *PackageRecord
    Symbols    []SymbolRecord
    References []ReferenceRecord
    Artifacts  []ArtifactRecord
}
```

Key struct definitions:

```go
type SymbolRecord struct {
    Name, QualifiedName, SymbolKind, Visibility, ParentSymbolID, StableID string
    StartLine, EndLine int
}
type ReferenceRecord struct {
    FromSymbolName, ToSymbolName, ReferenceKind, Confidence, RawTargetText string
    Line, ColumnStart, ColumnEnd int
}
type ArtifactRecord struct {
    ArtifactKind, Name, SymbolName, DataJSON, Confidence string
}
```

Fields and conventions relevant to new extractor implementation:
- **Reference kinds**: `imports`, `calls`, `registers_route`, `uses_config`, `touches_table`, `migrates`, `invokes_external_api`, `tests` (as `ReferenceRecord.ReferenceKind` string)
- **Artifact kinds**: `route`, `env_var`, `config_key`, `sql_query`, `migration`, `external_service`, `background_job` (as `ArtifactRecord.ArtifactKind` string)
- **Confidence levels**: `exact` (imports), `likely` (annotated routes/tests), `heuristic` (regex pattern matches)
- **Function signatures** for extraction helpers follow the pattern:
  ```
  extractCalls(content string, lines []string, codeLines []bool, symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord
  extractRoutes(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
  extractConfigAccess(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
  extractSQLArtifacts(content string, lines []string, filePath string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
  extractServices(content string, lines []string, codeLines []bool) ([]extractor.ReferenceRecord, []extractor.ArtifactRecord)
  extractTestReferences(symbols []extractor.SymbolRecord, moduleName string) []extractor.ReferenceRecord
  ```
- **Import references** produce `ReferenceRecord` with `ReferenceKind: "imports"`, `ToSymbolName` set to the import path, `Confidence: "exact"`, `Line` set to source line number, and `RawTargetText` set to the raw import path string.
- **Call extraction regexes** for all languages use the same two patterns:
  ```
  directCallRe   = (?m)(?:^|[^.\w])([a-z_]\w*)\s*\(   // captures function name
  selectorCallRe = (?m)(\w+)\.(\w+)\s*\(               // captures receiver.method
  ```
  Each language defines its own keyword exclusion set (see per-language sections). Policy: exclude all reserved keywords of the language. Contextual keywords (e.g., C#'s `value`, `get`, `set`) are not excluded since they can also be valid identifiers. The lists provided are intended to be comprehensive for reserved keywords; false positives from contextual keywords are acceptable at `heuristic` confidence.
- **SQL keywords** detected in string literals: `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `ALTER`, `DROP`, `FROM`, `WHERE`, `JOIN`, `TABLE`, `INDEX` (case-insensitive).
- **Route artifacts**: When a route is extracted, the `ArtifactRecord` has `ArtifactKind: "route"`, `Name` set to the route path (e.g., `/users/{id}`), and `DataJSON` set to a JSON object containing `{"method": "GET", "path": "/users/{id}", "handler": "handlerName"}` (handler may be empty if not captured). The corresponding `ReferenceRecord` has `ReferenceKind: "registers_route"` and `RawTargetText` set to `"METHOD /path"` (e.g., `"GET /users/{id}"`).
- **`test` as symbol kind**: A `test` is a distinct `SymbolKind` value, not an attribute on a method. When a method/function is identified as a test (via annotation, naming convention, or attribute), it is emitted as a symbol with `SymbolKind: "test"` rather than `SymbolKind: "method"`. This is consistent with existing extractors (Go, TypeScript, Python, Rust).
- **Extraction pipeline order**: The `Extract` method executes steps in this order: (1) imports, (2) symbols, (3) comment filter, (4) routes, (5) config, (6) SQL, (7) services, (8) calls (requires symbols), (9) test references (requires symbols). Steps 4-7 have no data dependencies on each other; they may execute in any order but all require codeLines from step 3.
- **Module name derivation**: Java uses the `package` declaration; C# uses the `namespace` declaration; Swift uses the file name (sans extension); Lua uses the file name (sans extension). If no declaration is found, the file name (without extension) is used as fallback.
- **Deduplication**: When multiple regexes match the same line producing the same artifact/reference kind and name, emit only once. Use a `seen` map keyed by `"line:kind:name"` to deduplicate, consistent with existing extractors. For SQL artifacts, deduplicate by start byte offset (as done in the Rust extractor).
- **Block end detection**: For brace-based languages (Java, C#, Swift), use brace-counting: starting from the declaration line, count `{` and `}` characters; when depth returns to 0, that line is the block end. This is the same algorithm used by `findBlockEnd` in the TypeScript and Rust extractors. Lua uses a different mechanism (see section 6.8).
- **Comment filter interface**: `commentfilter.LineFilter(content string, lang string) []bool` takes full file content and a language identifier string, returns a boolean slice where `codeLines[i]` is `true` if line `i+1` (1-indexed) is a code line and `false` if it is a comment-only line. This is used by all relationship extraction functions to skip matches inside comments.

---

## 3. Java Extractor

### 3.1 Package
`internal/extractor/javaextractor/`

### 3.2 File Extensions
`.java`

### 3.3 Supported Symbol Kinds
`package`, `class`, `interface`, `enum`, `method`, `field`, `const`, `annotation`, `test`

### 3.4 Import Extraction
```
importRe = (?m)^import\s+(?:static\s+)?([^;]+);
```
Produces `imports` references. Static imports detected but treated the same.

### 3.5 Symbol Extraction Regexes
```
packageRe   = (?m)^package\s+([\w.]+);
classDeclRe = (?m)^(?:public\s+|private\s+|protected\s+)?(?:abstract\s+)?(?:final\s+)?(?:static\s+)?class\s+(\w+)
interfaceRe = (?m)^(?:public\s+|private\s+|protected\s+)?interface\s+(\w+)
enumDeclRe  = (?m)^(?:public\s+|private\s+|protected\s+)?enum\s+(\w+)
methodDeclRe = (?m)^\s+(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:abstract\s+)?(?:final\s+)?(?:synchronized\s+)?(?:<[^>]+>\s+)?(?:\w+(?:<[^>]*>)?(?:\[\])*)\s+(\w+)\s*\(
constFieldRe = (?m)^\s+(?:public\s+|private\s+|protected\s+)?static\s+final\s+\w+\s+([A-Z_][A-Z0-9_]*)\s*=
annotationRe = (?m)^(?:public\s+)?@interface\s+(\w+)
```

### 3.6 Qualified Names
`packageName.ClassName.methodName` — dot-separated.

### 3.7 Visibility
Derived from access modifier keyword: `public` → `exported`, `private`/`protected`/default → `unexported`.

### 3.8 Block End Detection
Brace counting `{}` — same as TypeScript/Rust.

### 3.9 Test Detection
Two detection strategies:
1. Methods annotated with `@Test` or `@ParameterizedTest` (JUnit 4/5).
2. Methods starting with `test` in classes extending `TestCase` (JUnit 3).

```
junitTestRe  = (?m)@(?:Test|ParameterizedTest)\s*(?:\([^)]*\))?\s*\n\s*(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:void\s+)(\w+)
testCaseRe   = (?m)class\s+\w+\s+extends\s+TestCase   // detects TestCase subclass in file
testMethodRe = (?m)^\s+(?:public\s+)?void\s+(test\w+)\s*\(   // method names starting with "test"
```
When `testCaseRe` matches anywhere in the file, all methods matching `testMethodRe` are emitted as `test` symbols. Both strategies are applied independently and their results are merged. If a method matches both (e.g., annotated with `@Test` and named `testFoo`), it is emitted once (deduplicated by line number).

### 3.10 Route Extraction
| Framework | Pattern | Example |
|---|---|---|
| Spring MVC | `@GetMapping("/path")`, `@PostMapping`, `@RequestMapping` | `@GetMapping("/users/{id}")` |
| JAX-RS | `@GET`, `@POST` with `@Path("/path")` | `@Path("/users") @GET` |

```
springMappingRe = (?m)@(Get|Post|Put|Delete|Patch|Request)Mapping\s*\(\s*(?:value\s*=\s*)?["']([^"']+)["']
jaxrsPathRe     = (?m)@Path\s*\(\s*["']([^"']+)["']\s*\)
jaxrsMethodRe   = (?m)@(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)
```

### 3.11 Config Extraction
```
systemGetenvRe   = (?m)System\.getenv\s*\(\s*"([^"]+)"
systemPropertyRe = (?m)System\.getProperty\s*\(\s*"([^"]+)"
springValueRe    = (?m)@Value\s*\(\s*"\$\{([^}]+)\}"
```

### 3.12 Comment Filter
When `commentfilter.LineFilter(content, "java")` is called, it identifies `//` as single-line comments and non-nestable `/* */` as block comments.

### 3.13 Call/SQL/Service Patterns
- **Calls:** Uses shared `directCallRe`/`selectorCallRe` patterns (see section 2.3). Java keyword exclusion set: `abstract`, `assert`, `break`, `case`, `catch`, `class`, `continue`, `default`, `do`, `else`, `enum`, `extends`, `final`, `finally`, `for`, `if`, `implements`, `import`, `instanceof`, `interface`, `native`, `new`, `package`, `private`, `protected`, `public`, `return`, `static`, `strictfp`, `super`, `switch`, `synchronized`, `this`, `throw`, `throws`, `transient`, `try`, `volatile`, `while`, `yield`, `var`, `void`.
- **SQL:** String literals containing SQL keywords (see section 2.3 for keyword list). JDBC: any `execute`-prefixed method on Statement/PreparedStatement (`execute`, `executeQuery`, `executeUpdate`, `executeBatch`). JPA: `@Query("SQL")`.
  ```
  jpaQueryRe = (?m)@Query\s*\(\s*(?:value\s*=\s*)?["']([^"']+)["']
  ```
- **Services:** `HttpClient`, `RestTemplate`, `WebClient`, `OkHttpClient` → external_service. `ExecutorService`, `CompletableFuture`, `@Async`, `new Thread(` → background_job.

---

## 4. C# Extractor

### 4.1 Package
`internal/extractor/csharpextractor/`

### 4.2 File Extensions
`.cs`

### 4.3 Supported Symbol Kinds
`namespace`, `class`, `interface`, `enum`, `struct`, `method`, `property`, `field`, `const`, `record`, `test`

### 4.4 Import Extraction
```
usingRe = (?m)^using\s+(?:static\s+)?([^;]+);
```

### 4.5 Symbol Extraction Regexes
```
namespaceDeclRe = (?m)^namespace\s+([\w.]+)
classDeclRe     = (?m)^(?:\s*)(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:abstract\s+)?(?:sealed\s+)?(?:partial\s+)?class\s+(\w+)
interfaceDeclRe = (?m)^(?:\s*)(?:public\s+|private\s+|protected\s+|internal\s+)?(?:partial\s+)?interface\s+(\w+)
enumDeclRe      = (?m)^(?:\s*)(?:public\s+|private\s+|protected\s+|internal\s+)?enum\s+(\w+)
structDeclRe    = (?m)^(?:\s*)(?:public\s+|private\s+|protected\s+|internal\s+)?(?:readonly\s+)?(?:partial\s+)?struct\s+(\w+)
recordDeclRe    = (?m)^(?:\s*)(?:public\s+|private\s+|protected\s+|internal\s+)?(?:sealed\s+)?record\s+(?:struct\s+|class\s+)?(\w+)
methodDeclRe    = (?m)^\s+(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:virtual\s+|override\s+|abstract\s+|async\s+)?(?:\w+(?:<[^>]*>)?(?:\[\]|\?)?)\s+(\w+)\s*[(<]
constFieldRe    = (?m)^\s+(?:public\s+|private\s+|protected\s+|internal\s+)?const\s+\w+\s+(\w+)\s*=
propertyDeclRe  = (?m)^\s+(?:public\s+|private\s+|protected\s+|internal\s+)?(?:static\s+)?(?:virtual\s+|override\s+|abstract\s+)?(?:\w+(?:<[^>]*>)?(?:\[\]|\?)?)\s+(\w+)\s*\{
```

### 4.6 Qualified Names
`Namespace.ClassName.MethodName` — dot-separated.

### 4.7 Visibility
`public`/`internal` → `exported`, `private`/`protected` → `unexported`.

### 4.8 Test Detection
Methods with `[Test]` (NUnit), `[Fact]`/`[Theory]` (xUnit), or `[TestMethod]` (MSTest).
```
testAttrRe = (?m)\[(Test|Fact|Theory|TestMethod)\]\s*\n\s*(?:public\s+|private\s+)?(?:static\s+)?(?:async\s+)?(?:Task\s+|void\s+)(\w+)
```

### 4.9 Route Extraction
| Framework | Pattern | Example |
|---|---|---|
| ASP.NET | `[HttpGet("/path")]`, `[HttpPost]`, `[Route("/path")]` | `[HttpGet("users/{id}")]` |
| Minimal API | `app.MapGet("/path"`, `app.MapPost` | `app.MapGet("/users", handler)` |

```
aspnetRouteRe  = (?m)\[(Http(?:Get|Post|Put|Delete|Patch|Head|Options))\s*\(\s*"([^"]*)"
aspnetRouteRe2 = (?m)\[Route\s*\(\s*"([^"]+)"
minimalApiRe   = (?m)app\.Map(Get|Post|Put|Delete|Patch)\s*\(\s*"([^"]+)"
```

### 4.10 Config Extraction
```
envGetRe         = (?m)Environment\.GetEnvironmentVariable\s*\(\s*"([^"]+)"
configGetRe      = (?m)(?:configuration|config)\[["']([^"']+)["']\]
configGetSectRe  = (?m)(?:configuration|config)\.GetSection\s*\(\s*"([^"]+)"
configGetValRe   = (?m)(?:configuration|config)\.GetValue(?:<[^>]+>)?\s*\(\s*"([^"]+)"
```

### 4.11 Comment Filter
When `commentfilter.LineFilter(content, "csharp")` is called, it identifies `//` as single-line comments and non-nestable `/* */` as block comments.

### 4.12 Call/SQL/Service Patterns
- **Calls:** Uses shared `directCallRe`/`selectorCallRe` patterns (see section 2.3). C# keyword exclusion set: `abstract`, `as`, `base`, `break`, `case`, `catch`, `checked`, `class`, `continue`, `default`, `delegate`, `do`, `else`, `enum`, `event`, `explicit`, `extern`, `finally`, `fixed`, `for`, `foreach`, `goto`, `if`, `implicit`, `in`, `interface`, `internal`, `is`, `lock`, `namespace`, `new`, `operator`, `out`, `override`, `params`, `private`, `protected`, `public`, `readonly`, `ref`, `return`, `sealed`, `sizeof`, `stackalloc`, `static`, `struct`, `switch`, `this`, `throw`, `try`, `typeof`, `unchecked`, `unsafe`, `using`, `virtual`, `void`, `volatile`, `while`, `yield`, `var`, `await`, `async`, `record`.
- **SQL:** String literals/interpolated strings (`$"..."`) containing SQL keywords (see section 2.3 for keyword list). EF Core: `FromSqlRaw("SQL")`.
- **Services:** `HttpClient`, `WebRequest`, `RestClient` → external_service. `Task.Run(`, `Thread(`, `BackgroundService`, `Parallel.ForEach` → background_job.

---

## 5. Swift Extractor

### 5.1 Package
`internal/extractor/swiftextractor/`

### 5.2 File Extensions
`.swift`

### 5.3 Supported Symbol Kinds
`module`, `class`, `struct`, `enum`, `protocol`, `extension`, `function`, `method`, `property`, `const`, `test`

### 5.4 Import Extraction
```
importRe = (?m)^import\s+(\w+)
```

### 5.5 Symbol Extraction Regexes
```
classDeclRe    = (?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+|open\s+)?(?:final\s+)?class\s+(\w+)
structDeclRe   = (?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?struct\s+(\w+)
enumDeclRe     = (?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?(?:indirect\s+)?enum\s+(\w+)
protocolDeclRe = (?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?protocol\s+(\w+)
extensionDeclRe = (?m)^(?:public\s+|private\s+|internal\s+|fileprivate\s+)?extension\s+(\w+)
funcDeclRe     = (?m)^(?:\s*)(?:public\s+|private\s+|internal\s+|fileprivate\s+|open\s+)?(?:static\s+|class\s+)?(?:override\s+)?func\s+(\w+)
propertyDeclRe = (?m)^(?:\s*)(?:public\s+|private\s+|internal\s+|fileprivate\s+)?(?:static\s+|class\s+)?(?:lazy\s+)?(?:var|let)\s+(\w+)
```

### 5.6 Qualified Names
`ModuleName.ClassName.methodName` — dot-separated.

### 5.7 Visibility
`public`/`open` → `exported`, `private`/`fileprivate`/`internal`/(absence of explicit modifier) → `unexported`.

### 5.8 Test Detection
Methods starting with `test` in classes inheriting from `XCTestCase`, or functions with `@Test` (Swift Testing framework).
```
xctestRe    = (?m)^\s+(?:override\s+)?func\s+(test\w+)\s*\(
swiftTestRe = (?m)@Test\s*\n\s*(?:public\s+|private\s+)?func\s+(\w+)
```

### 5.9 Route Extraction
| Framework | Pattern | Example |
|---|---|---|
| Vapor | `app.get("path")`, `app.post("path")` | `app.get("users", ":id")` |

```
vaporRouteRe = (?m)(?:app|router|routes)\.(get|post|put|delete|patch)\s*\(\s*"([^"]+)"
```

### 5.10 Config Extraction
```
envGetRe     = (?m)ProcessInfo\.processInfo\.environment\[["']([^"']+)["']\]
envGetRe2    = (?m)Environment\.get\s*\(\s*"([^"]+)"
```

### 5.11 Comment Filter
When `commentfilter.LineFilter(content, "swift")` is called, it identifies `//` as single-line comments and nestable `/* */` as block comments.

### 5.12 Call/SQL/Service Patterns
- **Calls:** Uses shared `directCallRe`/`selectorCallRe` patterns (see section 2.3). Swift keyword exclusion set: `break`, `case`, `catch`, `class`, `continue`, `default`, `defer`, `do`, `else`, `enum`, `extension`, `fallthrough`, `for`, `func`, `guard`, `if`, `import`, `in`, `init`, `inout`, `is`, `let`, `operator`, `protocol`, `repeat`, `return`, `self`, `static`, `struct`, `subscript`, `super`, `switch`, `throw`, `throws`, `try`, `typealias`, `var`, `where`, `while`, `as`, `await`, `async`.
- **SQL:** String literals containing SQL keywords (see section 2.3 for keyword list). Core Data: `NSPredicate(format:)`, GRDB: `.filter(sql:)`.
- **Services:** `URLSession`, `Alamofire` → external_service. `DispatchQueue`, `Task {`, `async let` → background_job.

---

## 6. Lua Extractor

### 6.1 Package
`internal/extractor/luaextractor/`

### 6.2 File Extensions
`.lua`

### 6.3 Supported Symbol Kinds
`module`, `function`, `method`, `var`, `const`, `test`

### 6.4 Import Extraction
```
requireRe = (?m)(?:local\s+\w+\s*=\s*)?require\s*[\("]([^)"]+)[\)"]
```

### 6.5 Symbol Extraction Regexes
```
funcDeclRe      = (?m)^(?:local\s+)?function\s+(\w+(?:\.\w+)*)\s*\(
funcAssignRe    = (?m)^(?:local\s+)?(\w+)\s*=\s*function\s*\(
methodDeclRe    = (?m)^function\s+(\w+):(\w+)\s*\(
localVarRe      = (?m)^local\s+(\w+)\s*=
moduleVarRe     = (?m)^(\w+)\s*=\s*(?!function)
```

### 6.6 Qualified Names
`moduleName.functionName` — dot-separated. Methods: `moduleName.Class:method`.

### 6.7 Visibility
`local` → `unexported`, otherwise → `exported`.

### 6.8 Block End Detection
Lua uses `end` keyword for blocks. Starting from the declaration line, scan forward line by line. Increment depth by 1 for each occurrence of these block-opening keywords on a code line: `function`, `if`, `for`, `while`, `repeat`. The keyword `do` only increments depth when it appears at the start of a line (after optional whitespace) — i.e., a bare `do...end` block. The `do` that follows `for` or `while` on the same line does not increment depth (since `for`/`while` already did). `then`, `elseif`, and `else` do not change depth. Decrement depth by 1 on `end` (or `until` for `repeat...until`). Each `if...then...end` counts as one depth increment (from `if`) and one decrement (from `end`). Similarly, `for...do...end` counts as one increment (from `for`) and one decrement (from `end`). `then`, `elseif`, and `else` do not change depth. When depth reaches 0, that line is the block end. Comment-only lines (as identified by the comment filter) are skipped. Keywords inside string literals on code lines are not distinguished — this is a known limitation consistent with all existing regex-based extractors.

### 6.9 Test Detection
Functions starting with `test` (busted/luaunit convention), or `describe`/`it` blocks (busted BDD).
```
testFuncRe  = (?m)^(?:local\s+)?function\s+(test\w+)\s*\(
bustedDescRe = (?m)describe\s*\(\s*['"]([^'"]+)['"]
bustedItRe   = (?m)it\s*\(\s*['"]([^'"]+)['"]
```

### 6.10 Route Extraction
Lua has limited web framework adoption. Support:
- **OpenResty/Lapis:** `app:get("/path"`, `app:post("/path"`
- **Lapis:** `[path] = respond_to({GET = ...})`

```
lapisRouteRe = (?m)(?:app|self):?(get|post|put|delete)\s*\(\s*"([^"]+)"
```

### 6.11 Config Extraction
```
osGetenvRe = (?m)os\.getenv\s*\(\s*["']([^"']+)["']
```

### 6.12 Comment Filter
When `commentfilter.LineFilter(content, "lua")` is called, it identifies `--` as single-line comments and non-nestable `--[[ ]]` as block comments.

### 6.13 Call/SQL/Service Patterns
- **Calls:** Uses shared `directCallRe`/`selectorCallRe` patterns (see section 2.3). Lua keyword exclusion set: `and`, `break`, `do`, `else`, `elseif`, `end`, `false`, `for`, `function`, `goto`, `if`, `in`, `local`, `nil`, `not`, `or`, `repeat`, `return`, `then`, `true`, `until`, `while`.
- **SQL:** Lines containing SQL keywords (see section 2.3 for keyword list) on code lines (non-comment). Regex matches against the full line content; string literal boundary detection is not performed (consistent with existing extractors).
- **Services:** `http.request`, `socket.http` → external_service.
- **Jobs:** `ngx.timer.at`, `copas.addthread` → background_job.

---

## 7. Comment Filter Updates

`commentfilter.LineFilter` must accept three new language identifiers with the following comment syntax behavior:

| Language identifier | Single-line comment | Block comment | Nestable |
|---|---|---|---|
| `"java"`, `"csharp"` | `//` | `/* */` | No |
| `"swift"` | `//` | `/* */` | Yes |
| `"lua"` | `--` | `--[[ ]]` | No |

---

## 8. Registry and Configuration Updates

### 8.1 Registry
Add to `cmd/atlas/main.go`:
```go
reg.Register(javaextractor.New())
reg.Register(csharpextractor.New())
reg.Register(swiftextractor.New())
reg.Register(luaextractor.New())
```

### 8.2 Default Config
Add to default `include` globs in `.atlas/config.yaml`:
```yaml
- "**/*.java"
- "**/*.cs"
- "**/*.swift"
- "**/*.lua"
```

Add to `languages`:
```yaml
java: true
csharp: true
swift: true
lua: true
```

Add to default `exclude` globs:
```yaml
- "build/**"        # Java/Gradle
- "bin/**"          # C#
- "obj/**"          # C#
- ".build/**"       # Swift
- "Packages/**"     # Swift
```

---

## 9. Testing Strategy

Each extractor gets the same test structure as existing extractors:
- `{lang}_test.go` — unit tests for symbol/import extraction
- `routes_test.go` — route pattern tests
- `config_test.go` — config pattern tests
- `sql_test.go` — SQL detection tests
- `services_test.go` — service/job pattern tests
- `calls_test.go` — call extraction tests
- `tests_test.go` — test reference tests
- `TestExtract_FullPipeline` — integration test in main test file

---

## 10. README Update

Update two tables in `README.md`:

**Language Support table** — add one row per language (alphabetical order) with columns: Language, Parser (all "Regex/heuristic"), Symbols (list from each language's "Supported Symbol Kinds"), Relationships (imports, calls, tests, routes, config, SQL, services), Artifacts (routes, env vars, config keys, SQL queries, migrations, external services, background jobs).

**Framework detection table** — add rows:
| Language | Routes | HTTP Clients | Background Jobs |
|----------|--------|-------------|-----------------|
| C# | ASP.NET, Minimal API | HttpClient, WebRequest, RestClient | Task.Run, Thread, BackgroundService, Parallel |
| Java | Spring MVC, JAX-RS | HttpClient, RestTemplate, WebClient, OkHttp | ExecutorService, CompletableFuture, @Async, Thread |
| Lua | OpenResty/Lapis | http.request, socket.http | ngx.timer, copas |
| Swift | Vapor | URLSession, Alamofire | DispatchQueue, Task, async let |
