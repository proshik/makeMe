# Database schema

[← README](../README.md)

```
users
├── id              BIGSERIAL PK
├── telegram_id     BIGINT UNIQUE NOT NULL
├── username        TEXT
├── first_name      TEXT NOT NULL
├── level           TEXT NOT NULL  (A1/A2/B1/B2/C1/C2)
├── onboarded       BOOLEAN NOT NULL
├── created_at      TIMESTAMPTZ
├── updated_at      TIMESTAMPTZ
└── show_rule_on_correct BOOLEAN NOT NULL DEFAULT false  (web setting: rule after a correct answer too; the bot ignores it)

groups
├── id              BIGSERIAL PK
├── name            TEXT NOT NULL
├── invite_code     TEXT UNIQUE NOT NULL
├── created_by      BIGINT FK → users(id)
└── created_at      TIMESTAMPTZ

memberships
├── id              BIGSERIAL PK
├── group_id        BIGINT FK → groups(id) ON DELETE CASCADE
├── user_id         BIGINT FK → users(id) ON DELETE CASCADE
├── role            TEXT NOT NULL  (owner/member)
├── joined_at       TIMESTAMPTZ
└── UNIQUE(group_id, user_id)

assessment_sessions
├── id                  BIGSERIAL PK
├── user_id             BIGINT FK → users(id)
├── status              TEXT NOT NULL  (in_progress/completed)
├── current_level       TEXT NOT NULL  (A1–C2)
├── determined_level    TEXT           (A1–C2, NULL until completed)
├── pending_question    JSONB          (question shown to user, used to verify answer)
├── consecutive_correct INT NOT NULL   (correct answers in a row; 2 needed to promote level)
├── created_at          TIMESTAMPTZ
└── completed_at        TIMESTAMPTZ

assessment_answers
├── id              BIGSERIAL PK
├── session_id      BIGINT FK → assessment_sessions(id) ON DELETE CASCADE
├── question_level  TEXT NOT NULL  (A1–C2)
├── question_text   TEXT NOT NULL
├── options         JSONB NOT NULL
├── correct_option  INT NOT NULL
├── user_option     INT NOT NULL
├── correct         BOOLEAN NOT NULL
└── created_at      TIMESTAMPTZ

words
├── id              BIGSERIAL PK
├── english         TEXT NOT NULL
├── russian         TEXT NOT NULL
├── level           TEXT NOT NULL  (A1–C2)
├── example         TEXT
├── transcription   TEXT           (IPA, e.g. /ˈwɜːrd/, cached from Free Dictionary API)
└── audio_url       TEXT           (MP3 URL, cached from Free Dictionary API)

review_cards  (SRS / spaced repetition)
├── id              BIGSERIAL PK
├── user_id         BIGINT FK → users(id)
├── word_id         BIGINT FK → words(id)
├── ease_factor     REAL NOT NULL  (SM-2, default 2.5)
├── interval_days   INT NOT NULL
├── repetitions     INT NOT NULL
├── next_review_date DATE NOT NULL
├── updated_at      TIMESTAMPTZ
└── UNIQUE(user_id, word_id)

daily_sets
├── id              BIGSERIAL PK
├── user_id         BIGINT FK → users(id)
├── group_id        BIGINT FK → groups(id)
├── date            DATE NOT NULL
├── round           INT NOT NULL   (0-based; increments when current set is fully answered)
├── created_at      TIMESTAMPTZ
└── UNIQUE(user_id, group_id, date, round)

tasks
├── id              BIGSERIAL PK
├── daily_set_id    BIGINT FK → daily_sets(id)
├── type            TEXT NOT NULL  (vocab/grammar)
├── level           TEXT NOT NULL  (A1–C2)
├── prompt          TEXT NOT NULL
├── options         JSONB
├── answer          TEXT NOT NULL
├── metadata        JSONB
└── position        INT NOT NULL

task_results
├── id              BIGSERIAL PK
├── task_id         BIGINT FK → tasks(id)
├── user_id         BIGINT FK → users(id)
├── response        TEXT NOT NULL
├── correct         BOOLEAN NOT NULL
├── score           INT NOT NULL
├── skipped         BOOLEAN NOT NULL (true when user tapped "🤷 Не знаю")
├── duration_ms     INT
├── created_at      TIMESTAMPTZ
└── UNIQUE(task_id, user_id)

scores
├── id              BIGSERIAL PK
├── user_id         BIGINT FK → users(id)
├── group_id        BIGINT FK → groups(id)
├── date            DATE NOT NULL
├── raw             INT NOT NULL   (sum of task scores)
├── normalized      REAL NOT NULL  (raw × level coefficient)
├── created_at      TIMESTAMPTZ
├── updated_at      TIMESTAMPTZ
└── UNIQUE(user_id, group_id, date)

grammar_questions
├── id              BIGSERIAL PK
├── level           TEXT NOT NULL  (A1–C2)
├── category        TEXT NOT NULL  (Murphy topic, e.g. "Present Simple")
├── subcategory     TEXT NOT NULL  DEFAULT 'general'  (affirmative/negative/interrogative/spelling/general)
├── prompt          TEXT NOT NULL  (fill-in-the-blank with ___)
├── options         JSONB NOT NULL (array of 4 strings)
├── answer          TEXT NOT NULL  (correct option value)
└── UNIQUE(level, prompt)

grammar_categories
├── name            TEXT PK        (category name, e.g. "Present Simple")
└── explanation     TEXT NOT NULL  (theory shown in /learn)

word_learning_progress  (per-mode counters for /words)
├── user_id              BIGINT FK → users(id) ON DELETE CASCADE
├── word_id              BIGINT FK → words(id) ON DELETE CASCADE
├── en_ru_mc_correct     SMALLINT NOT NULL DEFAULT 0  (0..2)
├── ru_en_mc_correct     SMALLINT NOT NULL DEFAULT 0
├── en_ru_typing_correct SMALLINT NOT NULL DEFAULT 0
├── ru_en_typing_correct SMALLINT NOT NULL DEFAULT 0
├── learned              BOOLEAN NOT NULL DEFAULT FALSE
├── learned_at           TIMESTAMPTZ
├── started_at           TIMESTAMPTZ NOT NULL DEFAULT now()
├── updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
└── PK(user_id, word_id)

words_sessions  (the 10-word picks for /words; one open at a time)
├── id                BIGSERIAL PK
├── user_id           BIGINT FK → users(id) ON DELETE CASCADE
├── started_on        DATE NOT NULL
├── word_ids          BIGINT[] NOT NULL  (the 10 picks)
├── last_word_id      BIGINT             (anti-repeat cursor)
├── error_counts      JSONB NOT NULL DEFAULT '{}'   (word_id → mistakes in this session)
├── deferred_word_ids BIGINT[] NOT NULL DEFAULT '{}' (words excluded via "⏭ Отложить")
├── created_at        TIMESTAMPTZ
└── completed_at      TIMESTAMPTZ        (NULL while open; UNIQUE per user when NULL)

pending_typing_prompts  (waiting for the user's typed reply, both /words and /tasks)
├── user_id    BIGINT PK FK → users(id) ON DELETE CASCADE
├── chat_id    BIGINT NOT NULL
├── source     TEXT NOT NULL  ('words' | 'tasks')
├── payload    JSONB NOT NULL  ({session_id, word_id, mode} for words; {task_id} for tasks)
├── expires_at TIMESTAMPTZ NOT NULL
└── created_at TIMESTAMPTZ
```

Level coefficients for normalized scores: A1=0.6, A2=0.8, B1=1.0, B2=1.15, C1=1.3, C2=1.5

Relations:
- `users` 1 ←→ N `memberships` N ←→ 1 `groups`
- `users` 1 ←→ N `assessment_sessions` 1 ←→ N `assessment_answers`
- `users` 1 ←→ N `review_cards` N ←→ 1 `words`
- `users` 1 ←→ N `daily_sets` N ←→ 1 `groups`
- `daily_sets` 1 ←→ N `tasks` 1 ←→ 1 `task_results`
- `users` 1 ←→ N `word_learning_progress` N ←→ 1 `words`
- `users` 1 ←→ N `words_sessions`
- `users` 1 ←→ 0..1 `pending_typing_prompts`
- `groups.created_by` → `users.id`
