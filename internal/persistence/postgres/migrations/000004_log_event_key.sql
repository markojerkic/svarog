-- +goose Up
ALTER TABLE logs ADD COLUMN event_key bytea
    CHECK (event_key IS NULL OR octet_length(event_key) = 32);

CREATE UNIQUE INDEX logs_client_event_key_idx
    ON logs (client_id, event_key)
    WHERE event_key IS NOT NULL;

-- +goose Down
DROP INDEX logs_client_event_key_idx;
ALTER TABLE logs DROP COLUMN event_key;
