-- +goose Up
CREATE TABLE pending_typing_prompts (
    user_id    BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    chat_id    BIGINT NOT NULL,
    source     TEXT NOT NULL CHECK (source IN ('words','tasks')),
    payload    JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_pending_typing_expires ON pending_typing_prompts(expires_at);

-- +goose Down
DROP TABLE pending_typing_prompts;
