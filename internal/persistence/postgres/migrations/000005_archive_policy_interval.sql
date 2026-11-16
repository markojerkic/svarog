-- +goose Up
-- Replace legacy daily schedules with one full policy interval from this migration.
-- Hours match the age cutoff's fixed seven-day duration, including across DST.
UPDATE archive_policies
SET next_run_at = now() + archive_after_weeks * interval '168 hours';

-- +goose Down
-- Let the legacy scheduler start a fresh daily schedule on its next pass.
UPDATE archive_policies
SET next_run_at = now();
