-- Indexes on file_id columns that are used heavily in invalidation and
-- re-indexing (DELETE ... WHERE from_file_id = ?) but were not covered by
-- the initial migration. Without these, every per-file re-index does a full
-- scan of the references and artifacts tables.

CREATE INDEX IF NOT EXISTS idx_references_from_file_id ON "references"(from_file_id);
CREATE INDEX IF NOT EXISTS idx_references_to_file_id ON "references"(to_file_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_file_id ON artifacts(file_id);
