CREATE TABLE IF NOT EXISTS schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    language TEXT NOT NULL,
    package_name TEXT,
    module_name TEXT,
    content_hash TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    last_modified_utc TEXT,
    git_commit TEXT,
    is_generated INTEGER NOT NULL DEFAULT 0,
    parse_status TEXT NOT NULL CHECK (parse_status IN ('ok', 'error', 'partial', 'skipped')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS packages (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    import_path TEXT UNIQUE,
    directory_path TEXT NOT NULL UNIQUE,
    language TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS package_files (
    package_id INTEGER NOT NULL,
    file_id INTEGER NOT NULL,
    PRIMARY KEY (package_id, file_id),
    FOREIGN KEY(package_id) REFERENCES packages(id) ON DELETE CASCADE,
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS symbols (
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
    stable_id TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY(package_id) REFERENCES packages(id) ON DELETE SET NULL,
    FOREIGN KEY(parent_symbol_id) REFERENCES symbols(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS "references" (
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
    is_resolved INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    FOREIGN KEY(from_symbol_id) REFERENCES symbols(id) ON DELETE CASCADE,
    FOREIGN KEY(to_symbol_id) REFERENCES symbols(id) ON DELETE SET NULL,
    FOREIGN KEY(from_file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY(to_file_id) REFERENCES files(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS file_summaries (
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

CREATE TABLE IF NOT EXISTS package_summaries (
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

CREATE TABLE IF NOT EXISTS symbol_summaries (
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

CREATE TABLE IF NOT EXISTS artifacts (
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

CREATE TABLE IF NOT EXISTS index_runs (
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
    git_commit TEXT,
    notes TEXT
);

CREATE TABLE IF NOT EXISTS diagnostics (
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
    FOREIGN KEY(run_id) REFERENCES index_runs(id) ON DELETE CASCADE,
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE SET NULL
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_hash ON files(content_hash);
CREATE INDEX IF NOT EXISTS idx_packages_name ON packages(name);
CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
CREATE INDEX IF NOT EXISTS idx_symbols_qualified_name ON symbols(qualified_name);
CREATE INDEX IF NOT EXISTS idx_symbols_file_id ON symbols(file_id);
CREATE INDEX IF NOT EXISTS idx_references_from_symbol ON "references"(from_symbol_id);
CREATE INDEX IF NOT EXISTS idx_references_to_symbol ON "references"(to_symbol_id);
CREATE INDEX IF NOT EXISTS idx_references_kind ON "references"(reference_kind);
CREATE INDEX IF NOT EXISTS idx_artifacts_kind_name ON artifacts(artifact_kind, name);
CREATE INDEX IF NOT EXISTS idx_diagnostics_run_id ON diagnostics(run_id);
