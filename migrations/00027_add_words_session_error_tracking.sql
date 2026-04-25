-- +goose Up
ALTER TABLE words_sessions
    ADD COLUMN error_counts        JSONB    NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN deferred_word_ids   BIGINT[] NOT NULL DEFAULT '{}'::bigint[];

-- +goose Down
ALTER TABLE words_sessions
    DROP COLUMN error_counts,
    DROP COLUMN deferred_word_ids;
