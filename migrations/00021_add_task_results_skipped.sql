-- +goose Up
ALTER TABLE task_results ADD COLUMN skipped BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE task_results DROP COLUMN skipped;
