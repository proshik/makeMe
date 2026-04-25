-- +goose Up
CREATE TABLE words_sessions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    started_on   DATE NOT NULL,
    word_ids     BIGINT[] NOT NULL,
    last_word_id BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_words_sessions_user_open
    ON words_sessions(user_id) WHERE completed_at IS NULL;

-- +goose Down
DROP TABLE words_sessions;
