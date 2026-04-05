-- +goose Up
CREATE TABLE words (
    id       BIGSERIAL PRIMARY KEY,
    english  TEXT NOT NULL,
    russian  TEXT NOT NULL,
    level    TEXT NOT NULL CHECK (level IN ('A1','A2','B1','B2','C1','C2')),
    example  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_words_level ON words(level);

CREATE TABLE review_cards (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    word_id          BIGINT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    ease_factor      REAL NOT NULL DEFAULT 2.5,
    interval_days    INT NOT NULL DEFAULT 0,
    repetitions      INT NOT NULL DEFAULT 0,
    next_review_date DATE NOT NULL DEFAULT CURRENT_DATE,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, word_id)
);

CREATE INDEX idx_review_cards_user_due ON review_cards(user_id, next_review_date);

-- +goose Down
DROP TABLE review_cards;
DROP TABLE words;
