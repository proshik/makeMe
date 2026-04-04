-- +goose Up
CREATE TABLE assessment_sessions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress','completed')),
    current_level   TEXT NOT NULL DEFAULT 'B1' CHECK (current_level IN ('A1','A2','B1','B2','C1','C2')),
    determined_level TEXT CHECK (determined_level IN ('A1','A2','B1','B2','C1','C2')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE TABLE assessment_answers (
    id              BIGSERIAL PRIMARY KEY,
    session_id      BIGINT NOT NULL REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    question_level  TEXT NOT NULL CHECK (question_level IN ('A1','A2','B1','B2','C1','C2')),
    question_text   TEXT NOT NULL,
    options         JSONB NOT NULL,
    correct_option  INT NOT NULL,
    user_option     INT NOT NULL,
    correct         BOOLEAN NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_assessment_sessions_user_id ON assessment_sessions(user_id);
CREATE INDEX idx_assessment_answers_session_id ON assessment_answers(session_id);

-- +goose Down
DROP TABLE assessment_answers;
DROP TABLE assessment_sessions;
