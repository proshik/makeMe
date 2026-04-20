# makeMe

Gamified English learning platform via Telegram bot. Users join groups and compete daily by completing English tasks adapted to their level.

## Tech Stack

Go, PostgreSQL (pgx/sqlc/goose), chi, [go-telegram/bot](https://github.com/go-telegram/bot)

## Features

- **Adaptive assessment** — 30 adaptive questions in two phases: vocabulary (1–15) then grammar (16–30); starts at A1, requires 2 consecutive correct answers to promote a level; includes "I don't know" button and a full summary with level justification
- **Daily tasks** — 5 vocabulary + 5 grammar tasks per set; multiple sets per day (new round after completing the current one); solo mode — no group required, one is auto-created; resumes where you left off
- **Spaced repetition (SM-2)** — vocabulary cards follow the SM-2 algorithm; new words appear when reviews are caught up
- **Word bank** — ~900 words across A1–C2 with IPA transcription and audio pronunciation (fetched on demand from Free Dictionary API, cached in DB)
- **Grammar questions** — stored in DB with Murphy's Essential/English Grammar in Use categories; ~100 questions per A1–B1 level
- **Groups & leaderboards** — users join groups by invite code; daily/weekly/monthly leaderboards with normalized scores so different levels compete fairly
- **Fair scoring** — raw points multiplied by a level coefficient (A1=0.6 … C2=1.5) so a perfect A1 score ≈ a perfect C2 score
- **Profile** — current level, monthly points, daily streak, number of learned words
- **Proactive scheduler** — daily summary sent at 21:00 MSK; reassessment reminder after 30 days of inactivity
- **Docker support** — multi-stage `Dockerfile` + `docker-compose.yml` (app + PostgreSQL)

## Bot Commands

| Command | Description |
|---|---|
| `/start` | Register and see available commands. Returning users see their current level. |
| `/assess` | Take the level assessment test (adaptive, A1–C2, with "I don't know" button and final summary). |
| `/newgroup Название` | Create a new group. Returns an invite code to share with friends. |
| `/join КОД` | Join a group by invite code. |
| `/mygroups` | List all groups you belong to. |
| `/tasks` | Get today's tasks (3 vocab + 2 grammar). Generates if needed, resumes where you left off. |
| `/leaderboard` | Today's group leaderboard. Add `week` or `month` for other periods. |
| `/profile` | Your level, monthly score, daily streak, and number of learned words. |

## Typical Flow

```
User A (B2)                         User B (B1)
  │                                   │
  ├─ /start                           ├─ /start
  │  "Привет, Alice! ..."             │  "Привет, Bob! ..."
  │                                   │
  ├─ /assess                          ├─ /assess
  │  "Вопрос 1/10: She ___ a student" │  (10 adaptive questions, "Не знаю" button)
  │  [is] [are] [am] [be] [Не знаю]  │  ...
  │  ...10 questions...               │  "Тест завершён! Уровень: B1"
  │  "Тест завершён! Уровень: B2"     │  (full summary with correct/wrong per question)
  │  (level justification shown)      │
  │                                   │
  ├─ /newgroup English Club           │
  │  "Группа «English Club» создана!" │
  │  "Инвайт-код: a1b2c3d4"          │
  │                                   │
  │  ── shares code with Bob ──►      │
  │                                   ├─ /join a1b2c3d4
  │                                   │  "Ты вступил в группу «English Club»!"
  │                                   │
  ├─ /tasks                           ├─ /tasks
  │  "📖 Словарь — Задание 1/5"       │  "📝 Грамматика — Задание 1/5"
  │  "ephemeral  /ɪˈfem.ər.əl/"        │  "She ___ a teacher."
  │  [эфемерный] [вездесущий] ...     │  [is] [are] [am] [be]
  │  [🔊 Послушать]                   │
  │  "✅ Правильно! +10 баллов"       │  "❌ Неправильно. Ответ: is"
  │  ... (5 tasks) ...                │  ... (5 tasks) ...
  │  "Все задания выполнены! (5/5)"   │  "Все задания выполнены! (5/5)"
  │                                   │
  ├─ /leaderboard                     ├─ /leaderboard week
  │  "🏆 English Club — сегодня"      │  "🏆 English Club — неделя"
  │  "1. Alice  18.4 pts"             │  "1. Bob  62 pts"
  │  "2. Bob    12.0 pts"             │  "2. Alice  58 pts"
  │                                   │
  ├─ /profile                         │
  │  "Уровень: B2"                    │
  │  "Баллы за месяц: 186"            │
  │  "Серия: 7 дней подряд 🔥"        │
  │  "Выучено слов: 43"               │
  │                                   │
  ── 21:00 MSK — scheduler fires ──►  │
  │  "Итоги дня: 5/5 заданий ✅"      │  "Итоги дня: 3/5 заданий"
  │  "Ты на 1-м месте в English Club" │  "Ты на 2-м месте в English Club"
```

## Database Schema

```
users
├── id              BIGSERIAL PK
├── telegram_id     BIGINT UNIQUE NOT NULL
├── username        TEXT
├── first_name      TEXT NOT NULL
├── level           TEXT NOT NULL  (A1/A2/B1/B2/C1/C2)
├── onboarded       BOOLEAN NOT NULL
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ

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
```

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
├── question        TEXT NOT NULL
├── options         JSONB NOT NULL (array of 4 strings)
├── answer          INT NOT NULL   (0-based index)
└── created_at      TIMESTAMPTZ
```

Level coefficients for normalized scores: A1=0.6, A2=0.8, B1=1.0, B2=1.15, C1=1.3, C2=1.5

Relations:
- `users` 1 ←→ N `memberships` N ←→ 1 `groups`
- `users` 1 ←→ N `assessment_sessions` 1 ←→ N `assessment_answers`
- `users` 1 ←→ N `review_cards` N ←→ 1 `words`
- `users` 1 ←→ N `daily_sets` N ←→ 1 `groups`
- `daily_sets` 1 ←→ N `tasks` 1 ←→ 1 `task_results`
- `groups.created_by` → `users.id`

## Environment Variables

Copy `.env.example` to `.env` and fill in the values:

| Variable | Example | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable` | PostgreSQL connection string |
| `TELEGRAM_TOKEN` | `123456:ABC-DEF...` | Bot token from [@BotFather](https://t.me/BotFather) |
| `WEBHOOK_URL` | `https://example.com/webhook` | Public URL where Telegram sends updates. For local development use [ngrok](https://ngrok.com): `ngrok http 8080`, then set the resulting `https://xxxx.ngrok-free.app/webhook` |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `SCHEDULER_TIME` | `21:00` | Time to send daily summaries (HH:MM, 24h) |
| `TIMEZONE` | `Europe/Moscow` | Timezone for the scheduler |

## Development

```bash
cp .env.example .env   # fill in values (see table above)

make migrate-up        # run database migrations
make seed-words        # seed word bank from data/words.json (~900 words across A1–C2)
make sqlc              # regenerate sqlc code
make run               # start the server
make build             # build binary
```

### Word bank

The base word bank lives in `data/words.json` (~900 words, ~150 per CEFR level). To add words, edit the file and re-run `make seed-words` — existing entries are upserted, so no duplicates.

IPA transcription and audio pronunciation are fetched on demand from the [Free Dictionary API](https://dictionaryapi.dev/) (no key required) the first time a user taps **🔊 Послушать** on a vocabulary card, then cached in the DB.

## Testing

Integration tests use TestContainers (PostgreSQL + WireMock). Docker must be running.

```bash
go test ./internal/telegram/ -v -count=1
```
