-- +goose Up
-- Existing policies become due immediately; they were enabled but never scheduled.
ALTER TABLE archive_policies
    ADD COLUMN next_run_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE archive_policies DROP COLUMN next_run_at;
