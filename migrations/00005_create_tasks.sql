-- +goose Up
CREATE TABLE daily_sets (
    id        BIGSERIAL PRIMARY KEY,
    user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id  BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    date      DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, group_id, date)
);

CREATE TABLE tasks (
    id           BIGSERIAL PRIMARY KEY,
    daily_set_id BIGINT NOT NULL REFERENCES daily_sets(id) ON DELETE CASCADE,
    type         TEXT NOT NULL CHECK (type IN ('vocab','grammar')),
    level        TEXT NOT NULL CHECK (level IN ('A1','A2','B1','B2','C1','C2')),
    prompt       TEXT NOT NULL,
    options      JSONB,
    answer       TEXT NOT NULL,
    metadata     JSONB,
    position     INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_tasks_daily_set ON tasks(daily_set_id, position);

CREATE TABLE task_results (
    id          BIGSERIAL PRIMARY KEY,
    task_id     BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    response    TEXT NOT NULL,
    correct     BOOLEAN NOT NULL,
    score       INT NOT NULL DEFAULT 0,
    duration_ms INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(task_id, user_id)
);

-- +goose Down
DROP TABLE task_results;
DROP TABLE tasks;
DROP TABLE daily_sets;
