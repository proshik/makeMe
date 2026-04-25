-- +goose Up
CREATE TABLE word_learning_progress (
    user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    word_id              BIGINT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    en_ru_mc_correct     SMALLINT NOT NULL DEFAULT 0,
    ru_en_mc_correct     SMALLINT NOT NULL DEFAULT 0,
    en_ru_typing_correct SMALLINT NOT NULL DEFAULT 0,
    ru_en_typing_correct SMALLINT NOT NULL DEFAULT 0,
    learned              BOOLEAN NOT NULL DEFAULT FALSE,
    learned_at           TIMESTAMPTZ,
    started_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, word_id)
);
CREATE INDEX idx_wlp_user_active  ON word_learning_progress(user_id) WHERE learned = false;
CREATE INDEX idx_wlp_user_learned ON word_learning_progress(user_id) WHERE learned = true;

-- +goose Down
DROP TABLE word_learning_progress;
