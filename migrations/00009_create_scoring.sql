-- +goose Up
CREATE TABLE scores (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    group_id    BIGINT NOT NULL REFERENCES groups(id),
    period      TEXT NOT NULL CHECK (period IN ('day', 'month')),
    period_key  TEXT NOT NULL,
    raw         INT NOT NULL DEFAULT 0,
    normalized  FLOAT8 NOT NULL DEFAULT 0,
    UNIQUE (user_id, group_id, period, period_key)
);

-- +goose Down
DROP TABLE scores;
