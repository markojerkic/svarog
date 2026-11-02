-- +goose NO TRANSACTION
-- +goose Up
-- CONCURRENTLY keeps log ingestion available while the indexes are changed.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS logs_message_trgm_idx
    ON logs USING gin(message gin_trgm_ops);

DROP INDEX CONCURRENTLY IF EXISTS logs_search_idx;

ANALYZE logs;

-- +goose Down
CREATE INDEX CONCURRENTLY IF NOT EXISTS logs_search_idx
    ON logs USING gin(to_tsvector('simple'::regconfig, message));

DROP INDEX CONCURRENTLY IF EXISTS logs_message_trgm_idx;

ANALYZE logs;
