# makeMe

Gamified English learning platform via Telegram bot. Users join groups and compete daily by completing English tasks adapted to their level.

## Tech Stack

Go, PostgreSQL (pgx/sqlc/goose), chi, [go-telegram/bot](https://github.com/go-telegram/bot)

## Features

- **Adaptive assessment** — 30 adaptive questions in two phases: vocabulary (1–15) then grammar (16–30); starts at A1, requires 2 consecutive correct answers to promote a level; includes "I don't know" button and a full summary with level justification
- **Daily tasks** — 5 vocabulary + 5 grammar tasks per set; up to 5 rounds per day per user (configurable via `MAX_ROUNDS_PER_DAY`); solo mode — no group required, one is auto-created; resumes where you left off. Each task shows a **🤷 Не знаю** button — honest skips are recorded separately and treated as wrong for SRS scheduling but don't penalise beyond 0 points.
- **Spaced repetition (SM-2)** — vocabulary cards follow the SM-2 algorithm; new words appear when reviews are caught up
- **Word bank** — 1100 words across A1–C2 (A1 292, A2 200, B1 166, B2 146, C1 149, C2 147) with IPA transcription and audio pronunciation (fetched on demand from Free Dictionary API, cached in DB)
- **Grammar questions** — stored in DB with Murphy's English Grammar in Use categories; 768 questions across A1–C2 over 113 categories; each tagged with subcategory (affirmative/negative/interrogative/spelling/general) for balanced `/learn` sessions
- **Groups & leaderboards** — users join groups by invite code; daily and monthly leaderboards with normalized scores so different levels compete fairly (weekly is planned, not implemented)
- **Fair scoring** — raw points multiplied by a level coefficient (A1=0.6 … C2=1.5) so a perfect A1 score ≈ a perfect C2 score
- **Profile** — current level, monthly points, daily streak, number of learned words
- **Proactive scheduler** — daily summary sent at `SCHEDULER_TIME` (default 21:00 in `TIMEZONE`, default Europe/Moscow); reassessment reminder to users whose last completed assessment is more than 30 days old
- **Web UI** — browser interface at `/` with Telegram login (OAuth 2.0 + PKCE), sharing the same domain services and the same Postgres state as the bot: start in the browser, continue in Telegram. Design system «Табло» (`design-concepts/tablo.html`). Assets are vendored, not loaded from a CDN.
- **Docker support** — multi-stage `Dockerfile` + `docker-compose.yml` (app + PostgreSQL)

## Bot Commands

| Command | Description |
|---|---|
| `/start` | Register and see available commands. Returning users see their current level. |
| `/help` | The same "how it works" walkthrough `/start` shows, plus the full command list. |
| `/assess` | Take the level assessment test (adaptive, A1–C2, with "I don't know" button and final summary). |
| `/newgroup Название` | Create a new group. Returns an invite code to share with friends. |
| `/join КОД` | Join a group by invite code. |
| `/mygroups` | List all groups you belong to. |
| `/tasks` | Get today's tasks (5 vocab + 5 grammar). Generates if needed, resumes where you left off. Vocab tasks now include MC modes (en→ru, ru→en, fill-blank) plus typing modes where you reply with the translation as a text message. |
| `/words` | Active-recall learning mode (does NOT count toward the leaderboard). Each session presents 10 words drawn **strictly bottom-up by CEFR level** — A1 first, then A2, … up to C2 — regardless of your profile level. The next level only feeds in when the current one is exhausted (every word either learned or in progress). The intro screen shows a per-CEFR-level progress block (`A1: 12/120 (10%)`, …) — counts are computed live from the `words` table, so seeding new entries via `data/words.json` updates the denominator on the next session. The "✅ Я уже знаю это слово" button is the explicit way to retire an A1 word without going through the full 4-mode drill, so a B2 user typically clicks through familiar A1s quickly the first time around. Each word is drilled in 4 modes — EN→RU MC, RU→EN MC, EN→RU typing, RU→EN typing — and is "learned" once you have answered correctly twice in every mode. A wrong answer resets the per-mode counter. Once learned, the word joins the SM-2 cycle in `/tasks`. There's no per-day cap on sessions — finish one and run `/words` again for a new set. If a particular word just isn't sticking and you accumulate more than 5 mistakes on it within the session (`words.DeferThreshold = 5`), a "⏭ Отложить это слово до завтра" button appears in the feedback — tap it to exclude that word from the rest of this session (progress is preserved; the word reappears in your next /words session). |
| `/learn` | Browse grammar topics at your level (and the next one). Each practice session shows up to 20 questions (`learnPracticeLimit`) with a stratified mix of subtypes (affirmative / negative / interrogative / spelling / general). A fresh random seed is drawn every time you open a topic, so the question order is reshuffled on each entry and stays stable for the duration of that run. |
| `/leaderboard` | Today's group leaderboard. Registered as an exact match — it takes **no text arguments**; a "📅 За месяц" inline button under the table toggles between today and the current month. With several groups you first pick one. Only `day` and `month` periods exist (scores are stored under those two keys); there is no weekly leaderboard yet — see plan phase 13. |
| `/profile` | Your level, monthly score, daily streak, words stable in spaced repetition (3+ reps), words learned via `/words`, how many are still in progress, plus a per-CEFR-level progress block (`A1: 12/120 (10%)`, …) showing how many words from the corpus you've learned at each level. |

## Typical Flow

```
User A (B2)                         User B (B1)
  │                                   │
  ├─ /start                           ├─ /start
  │  "Привет, Alice! ..."             │  "Привет, Bob! ..."
  │                                   │
  ├─ /assess                          ├─ /assess
  │  "Вопрос 1/30: She ___ a student" │  (30 adaptive questions, "Не знаю" button)
  │  [is] [are] [am] [be] [Не знаю]  │  ...
  │  ...30 questions...               │  "Тест завершён! Уровень: B1"
  │  (1–15 vocab, 16–30 grammar)      │  (full summary with correct/wrong per question)
  │  "Тест завершён! Уровень: B2"     │
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
  │  "📚 Учим слова"                  │
  │  "Слов в сессии: 10"              │
  │  "📖 EN→RU выбор · показ 1/80"    │
  │  (no word index — it would spoil)  │
  │  [приоритет] [главный] ...        │
  │  → en_ru_mc count = 1             │
  │  ... 4 modes per word, twice ...  │
  │  "🎉 Слово priority выучено!"     │
  │   (auto-promoted to SM-2)         │
  │                                   │
  ├─ /leaderboard                     ├─ /leaderboard → [📅 За месяц]
  │  "🏆 Группа «English Club» —      │  "🏆 Группа «English Club» —
  │   сегодня"                        │   месяц 2026-07"
  │  "🥇 Alice — 18.4 очков"          │  "🥇 Bob — 62.0 очков"
  │  "🥈 Bob — 12.0 очков"            │  "🥈 Alice — 58.0 очков"
  │  [📅 За месяц]                    │  [📅 За сегодня]
  │                                   │
  ├─ /profile                         │
  │  "🎓 Уровень: B2"                 │
  │  "⭐ Баллы за месяц: 186.0 баллов"│
  │  "🎽 Дней подряд занимаешься: 7🔥"│
  │  "📚 Слова"                       │
  │  "🧠 В интервальных повторениях:43"│
  │  "✍️ Усвоено через /words: 61"    │
  │  "📖 В работе: 8"                 │
  │  "📊 По уровням: A1: 120/292 …"   │
  │                                   │
  ── 21:00 MSK — scheduler fires ──►  │
  │  "📊 Итоги дня, Alice!"           │  "📊 Итоги дня, Bob!"
  │  "English Club:"                  │  "English Club:"
  │  "  Баллы: 46 (52.9 норм.)"       │  "  Баллы: 40 (40.0 норм.)"
  │  "  Место: 1 из 2"                │  "  Место: 2 из 2"
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

## Environment Variables

Copy `.env.example` to `.env` and fill in the values:

| Variable | Example | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable` | PostgreSQL connection string |
| `TELEGRAM_TOKEN` | `123456:ABC-DEF...` | Bot token from [@BotFather](https://t.me/BotFather) |
| `WEBHOOK_URL` | `https://example.com/webhook` | Public URL where Telegram sends updates. For local development use [ngrok](https://ngrok.com): `ngrok http 8080`, then set the resulting `https://xxxx.ngrok-free.app/webhook` |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `WORDS_FILE` | `data/words.json` | Word list seeded on every startup |
| `GRAMMAR_QUESTIONS_FILE` | `data/grammar.json` | Grammar questions seeded on every startup |
| `GRAMMAR_CATEGORIES_FILE` | `data/grammar_categories.json` | Grammar category explanations (`/learn` theory) seeded on every startup |
| `SCHEDULER_TIME` | `21:00` | Time to send daily summaries (HH:MM, 24h) |
| `TIMEZONE` | `Europe/Moscow` | Timezone for the scheduler |
| `MAX_ROUNDS_PER_DAY` | `5` | Max task rounds a user can run per day (one round = 10 tasks) |
| `ADMIN_TOKEN` | `(empty)` | Bearer token for `/admin/*` HTTP endpoints. Leave empty to disable them (they return 503). Generate with `openssl rand -hex 32`. |
| `APP_BASE_URL` | `https://makeme.example.com` | Public origin of the web UI. `redirect_uri` is derived as `$APP_BASE_URL/auth/telegram/callback` — register that exact URL with Telegram. |
| `TELEGRAM_CLIENT_ID` | `(empty)` | OAuth client id. Separate from `TELEGRAM_TOKEN`. |
| `TELEGRAM_CLIENT_SECRET` | `(empty)` | OAuth client secret. |
| `JWT_SECRET` | `(empty)` | Base64 key for session JWTs, ≥32 bytes decoded. `openssl rand -base64 32`. Startup fails if it is shorter or not base64. |
| `JWT_TTL_HOURS` | `24` | Session lifetime; the `auth_token` cookie max-age is derived from it. |
| `COOKIE_SECURE` | `true` | Set `false` only for local http development. |
| `TELEGRAM_OAUTH_BASE_URL` | `https://oauth.telegram.org` | OIDC provider base. Override only in tests. |

The web UI is optional: with `APP_BASE_URL`, `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET` or `JWT_SECRET` unset it is not mounted and only the bot runs. A warning is logged at startup saying which variables are missing.

## Web UI

| Route | Description |
|---|---|
| `GET /login` | Sign-in page. Redirects to `/` if already signed in. |
| `GET /auth/telegram` | Starts OAuth: mints `state` + PKCE verifier into 5-minute HttpOnly cookies, redirects to Telegram. |
| `GET /auth/telegram/callback` | Verifies `state`, exchanges the code for an `id_token`, resolves the Telegram identity via `user.Service.Register` (idempotent by `telegram_id`, so bot users keep their row), sets the `auth_token` cookie. |
| `GET /logout` | Clears the session cookie. |
| `GET /` | Home. Requires auth. |
| `GET /tasks` | Today's round. Generates or resumes it, auto-creating a solo group if the user has none — the same rule the bot follows. |
| `GET /tasks/next` | Next slot as an htmx fragment. Deliberately reads the *current* set instead of generating: `GenerateDaily` opens the next round once the current one is answered, so generating here would roll the user into a new round instead of showing the summary. |
| `POST /tasks/{id}/answer` | Records a choice, a typed answer or a skip via `task.Service`, scores it, and returns the feedback fragment. |
| `GET /assess` | Starts or resumes the 30-question level test. The current level is deliberately not shown during the run — it moves adaptively after every answer, and showing it would turn the test into a game against the system. |
| `POST /assess/answer` | Records an option or a skip, returns the next question or the result — level, score, and a per-question breakdown so the verdict is justified. |
| `GET /words` | Opens or resumes the ten-word active-recall session, with per-mode tallies and live per-CEFR-level progress. |
| `GET /words/next` | Next drill as a fragment. Completing the last one closes the session, so the following visit picks a fresh ten. |
| `POST /words/{id}/answer` | Records a choice, a typed answer or a skip. Multiple-choice correctness is decided server-side from the submitted text — the correct index is never sent to the browser. |
| `POST /words/{id}/know` | Retires a word the user already knows across all four modes and drops it into spaced repetition. |
| `POST /words/{id}/defer` | Drops a word from the rest of the session, keeping its progress. Offered only past `words.DeferThreshold` mistakes on that word. |

| `GET /learn` | Grammar topics at the user's level and the one above it. |
| `GET /learn/topic?name=…` | Theory plus a fresh random seed, so re-entering a topic reshuffles the order. |
| `GET /learn/practice?name=…&seed=…&i=…` | One question of a run. The run keeps no server-side state: seed plus index reproduce the same sample, so a refresh does not lose the user's place. |
| `POST /learn/answer` | Checks the answer and returns the feedback fragment. The rule is shown only when the answer was wrong — after a correct one it is noise. |

Topic names travel as query values rather than path segments: categories like `can/can't` carry slashes, which a path segment would mangle.

`/learn` and `/words` write no score rows — not contributing to the leaderboard is its one behavioural difference from `/tasks`, and a test guards it.
| `GET /static/*` | Self-hosted assets, served from the binary via `embed.FS`. |

Asset URLs carry a `?v=` token derived from a hash of every embedded file, so `Cache-Control: immutable` is safe — a new build produces new URLs. Without it a deploy would leave browsers on the previous stylesheet for a year.

Session cookies are `HttpOnly`, `Secure`, `SameSite=Lax` — Lax is the CSRF defence for cookie auth, which is why the POST handlers carry no separate CSRF token. The `id_token` signature is deliberately not verified: it arrives from Telegram's token endpoint over TLS server-to-server, and there is no public JWKS to check against; `exp`, `aud` and `iss` are validated as defence in depth.

### Assets are self-hosted, and Tailwind is pre-built

Everything the pages load comes from our own origin (`internal/web/static/`, embedded into the binary): the built Tailwind stylesheet, htmx 2.0.4, and the four fonts of the «Табло» system — Oswald, IBM Plex Sans, IBM Plex Mono, and Charis SIL for IPA transcriptions.

Two separate decisions here, for two separate reasons.

**Nothing is loaded from a CDN.** A CDN means depending on someone else's uptime and — the deciding factor — being unable to close the CSP. With everything local the app ships `default-src 'self'; script-src 'self'`, with no `'unsafe-inline'` for scripts. That in turn constrains the templates: no inline `<script>`, no `on*=""` handlers, no htmx `hx-on` / `js:` expressions — behaviour lives in `static/js/app.js` and hangs off `data-action` attributes. Four tests guard all of this: the CSP directives, the absence of `unsafe-inline` in `script-src`, the absence of inline scripts and handlers in rendered pages, and any page referencing an external host.

`style-src` does keep `'unsafe-inline'`: templates carry inline `style=""` for design-token colours, and style injection is far less dangerous than script execution — an accepted trade-off rather than a nonce pipeline.

**Tailwind is compiled ahead of time, not in the browser.** `cdn.tailwindcss.com` is the Play CDN, which Tailwind documents as development-only: it ships the compiler and generates CSS on every page load. Instead, `make css` runs the standalone Tailwind CLI (a single binary — no Node, no `npm install`) over `internal/web/templates/**` and writes `internal/web/static/tailwind.css`. That output is **committed**, because `//go:embed` needs the file present at compile time — which also means `docker build` stays a plain Go build with no asset step.

```bash
make css         # rebuild after editing templates
make css-check   # fails if the committed CSS is stale (runs in CI)
```

`tailwind.config.js` zeroes the `borderRadius` and `boxShadow` scales outright rather than via `extend`, so `rounded-lg` and `shadow-md` do not exist at all — the design system forbids radii and shadows, and a test asserts they never appear in the built CSS.

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

make install-tools     # install sqlc + goose (uses the public Go proxy)
make sqlc              # regenerate sqlc code after changing queries/
make css               # rebuild the web stylesheet after editing templates
make run               # start the server (applies migrations + seeds on startup)
make build             # build binary

make migrate-up        # apply migrations manually (the app also does this on startup)
make migrate-down      # roll back one migration
make migrate-create name=add_something   # scaffold a new goose migration
```

### Seed data

Three JSON files are seeded automatically every time the application starts — no separate command needed:

| File | Contents | Count |
|---|---|---|
| `data/words.json` | vocabulary (english/russian/level/example/transcription) | 1100 |
| `data/grammar.json` | grammar questions (level/category/subcategory/prompt/options/answer) | 768 |
| `data/grammar_categories.json` | per-category theory shown in `/learn` | 113 |

To add content, edit the relevant JSON file and restart the app; upserts are idempotent. A missing or malformed file is logged as a warning and skipped — startup still succeeds.

`cmd/seed` is a standalone CLI for loading one of these files into the DB without starting the server (`go run ./cmd/seed data/words.json`). It is redundant with the startup seeder and only useful for one-off imports against a remote database.

IPA transcription and audio pronunciation are fetched on demand from the [Free Dictionary API](https://dictionaryapi.dev/) (no key required) the first time a user taps **🔊 Послушать** on a vocabulary card, then cached in the DB.

## Testing

Integration tests in `internal/telegram/` use TestContainers (PostgreSQL + WireMock 3.13.0). Docker must be running for those. Every other package has plain unit tests and needs no Docker.

```bash
go test ./internal/telegram/ -v -count=1   # integration suite (needs Docker)
go test -race ./...                        # everything — this is what CI runs
```

CI (`.github/workflows/ci.yml`) runs `go test -v -race ./...` on Go 1.26 for every push and PR to `master`.
