-- +goose Up
CREATE TABLE grammar_questions (
    id       BIGSERIAL PRIMARY KEY,
    level    TEXT NOT NULL CHECK (level IN ('A1','A2','B1','B2','C1','C2')),
    category TEXT NOT NULL,
    prompt   TEXT NOT NULL,
    options  JSONB NOT NULL,
    answer   TEXT NOT NULL
);

CREATE INDEX ON grammar_questions(level);

-- +goose Down
DROP TABLE grammar_questions;
