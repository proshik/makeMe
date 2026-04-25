# makeMe

Gamified English learning platform via Telegram bot. Users join groups and compete daily by completing English tasks adapted to their level.

## Tech Stack

Go, PostgreSQL (pgx/sqlc/goose), chi, [go-telegram/bot](https://github.com/go-telegram/bot)

## Features

- **Adaptive assessment** — 30 adaptive questions in two phases: vocabulary (1–15) then grammar (16–30); starts at A1, requires 2 consecutive correct answers to promote a level; includes "I don't know" button and a full summary with level justification
- **Daily tasks** — 5 vocabulary + 5 grammar tasks per set; up to 5 rounds per day per user (configurable via `MAX_ROUNDS_PER_DAY`); solo mode — no group required, one is auto-created; resumes where you left off. Each task shows a **🤷 Не знаю** button — honest skips are recorded separately and treated as wrong for SRS scheduling but don't penalise beyond 0 points.
- **Spaced repetition (SM-2)** — vocabulary cards follow the SM-2 algorithm; new words appear when reviews are caught up
- **Word bank** — ~900 words across A1–C2 with IPA transcription and audio pronunciation (fetched on demand from Free Dictionary API, cached in DB)
- **Grammar questions** — stored in DB with Murphy's English Grammar in Use categories; 600+ questions across A1–C2; each tagged with subcategory (affirmative/negative/interrogative) for balanced `/learn` sessions
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
| `/tasks` | Get today's tasks (5 vocab + 5 grammar). Generates if needed, resumes where you left off. Vocab tasks now include MC modes (en→ru, ru→en, fill-blank) plus typing modes where you reply with the translation as a text message. |
| `/words` | Active-recall learning mode (does NOT count toward the leaderboard). Each session presents 10 words (5 from your level + 5 from the next level). Each word is drilled in 4 modes — EN→RU MC, RU→EN MC, EN→RU typing, RU→EN typing — and is "learned" once you have answered correctly twice in every mode. A wrong answer resets the per-mode counter. Once learned, the word joins the SM-2 cycle in `/tasks`. There's no per-day cap on sessions — finish one and run `/words` again for a new set. If a particular word just isn't sticking and you accumulate more than 5 mistakes on it within the session, a "⏭ Defer until tomorrow" button appears in the feedback — tap it to exclude that word from the rest of this session (progress is preserved; the word reappears in your next /words session). |
| `/learn` | Browse grammar topics at your level (and the next one). Each practice session shows up to 20 questions with a stratified mix of subtypes (affirmative / negative / interrogative / spelling). |
| `/leaderboard` | Today's group leaderboard. Add `week` or `month` for other periods. |
| `/profile` | Your level, monthly score, daily streak, words stable in SM-2, words usvoено via /words, and how many words are still in progress. |

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
  │  "📖 Словарь — Задание 1/10"      │  "📝 Грамматика — Задание 1/10"
  │  "ephemeral  /ɪˈfem.ər.əl/"       │  "She ___ a teacher."
  │  [эфемерный] [вездесущий] ...     │  [is] [are] [am] [be]
  │  [🔊 Послушать]                   │
  │  "✅ Правильно! +11.5 баллов"     │  "❌ Неправильно. Ответ: is"
  │  ─ task 3: typing subtype ─        │  ... (10 tasks) ...
  │  "✍️ Введи перевод: HOUSE"        │
  │  Bob types: "дом" → ✅            │
  │  ... (10 tasks) ...                │
  │  "Все задания выполнены! (10/10)" │
  │                                   │
  ├─ /words                           │
  │  "📚 Учим слова — 10 слов"        │
  │  "📖 1/10 · EN→RU выбор"          │
  │  [приоритет] [главный] ...        │
  │  → en_ru_mc count = 1             │
  │  ... 4 modes per word, twice ...  │
  │  "🎉 Слово priority выучено!"     │
  │   (auto-promoted to SM-2)         │
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
├── id              BIGSERIAL PK
├── user_id         BIGINT FK → users(id) ON DELETE CASCADE
├── started_on      DATE NOT NULL
├── word_ids        BIGINT[] NOT NULL  (the 10 picks)
├── last_word_id    BIGINT             (anti-repeat cursor)
├── created_at      TIMESTAMPTZ
└── completed_at    TIMESTAMPTZ        (NULL while open; UNIQUE per user when NULL)

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
| `MAX_ROUNDS_PER_DAY` | `5` | Max task rounds a user can run per day (one round = 10 tasks) |
| `ADMIN_TOKEN` | `(empty)` | Bearer token for `/admin/*` HTTP endpoints. Leave empty to disable them (they return 503). Generate with `openssl rand -hex 32`. |

## Admin Endpoints

Protected by `Authorization: Bearer $ADMIN_TOKEN`. Disabled when `ADMIN_TOKEN` is empty (responds 503).

### `POST /admin/seed-audio`

Bulk-fetches transcription + audio_url from the Free Dictionary API for words at the given CEFR levels and persists them to the `words` table. Idempotent: words that already have `audio_url` are skipped.

Body (JSON):
```json
{ "levels": ["A1", "A2"], "delay_ms": 500 }
```

Response: streamed `application/x-ndjson`, one line per event (start of level / per-word result / final summary). Use `curl -N` so output isn't buffered:

```bash
curl -N -X POST https://your-host.example.com/admin/seed-audio \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"levels":["A1","A2"],"delay_ms":500}'
```

For local seeding without HTTP, the same logic ships as `cmd/seed-audio`:
```bash
go run ./cmd/seed-audio -levels=A1,A2 -delay-ms=500
```

#### Production workflow

`audio_url` is **not** part of `data/words.json` and is **not** copied across environments by the on-startup seeder (`UpsertWord` only touches english/russian/level/example/transcription). After deploying to production, populate audio with:

1. Generate and set a token (do this once, store in your secret manager):
   ```bash
   openssl rand -hex 32   # → 64 hex chars
   ```
   Set the value as `ADMIN_TOKEN` in the production environment and redeploy so the service picks it up.

2. From any machine with HTTPS access to prod, run:
   ```bash
   curl -N -X POST https://your-prod.example.com/admin/seed-audio \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"levels":["A1","A2"],"delay_ms":500}'
   ```
   Stream of ndjson events ends with a `{"event":"done", ...}` summary line. ~5 minutes for A1+A2 (~492 words at 500 ms delay).

3. **If 429 appears** (`"status":"failed","error":"unexpected status: 429"`) — Free Dictionary throttled you. Wait 1–2 minutes and re-run with the same body; the run is idempotent so already-populated rows are skipped.

4. **Adding more levels** later (B1–C2): same call, change `levels`. ~608 words across B1–C2, allow ~10 minutes total.

#### Limits to expect

* Free Dictionary returns no audio at all for ~3–5% of common words (e.g. `look`, `room`, `sea`); they end up as `"status":"not_found"` and stay with empty `audio_url`. The user-facing `🔊 Послушать` button will reply "Произношение не найдено" for these — acceptable for v1.
* Disconnect (closing curl, network drop) is handled: the server cancels the run via `ctx.Done()` so no quota is wasted on a dropped client.

## Development

```bash
cp .env.example .env   # fill in values (see table above)

make sqlc              # regenerate sqlc code after changing queries/
make run               # start the server (applies migrations + seeds on startup)
make build             # build binary
```

### Seed data

Words (`data/words.json`) and grammar questions (`data/grammar.json`) are seeded automatically every time the application starts — no separate command needed. To add content, edit the relevant JSON file and restart the app; upserts are idempotent.

IPA transcription and audio pronunciation are fetched on demand from the [Free Dictionary API](https://dictionaryapi.dev/) (no key required) the first time a user taps **🔊 Послушать** on a vocabulary card, then cached in the DB.

## Testing

Integration tests use TestContainers (PostgreSQL + WireMock). Docker must be running.

```bash
go test ./internal/telegram/ -v -count=1
```
