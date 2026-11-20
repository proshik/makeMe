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
- **Web UI** — browser interface at `/` with Telegram login (OAuth 2.0 + PKCE), sharing the same domain services and the same Postgres state as the bot: start in the browser, continue in Telegram. Design system «Табло», defined in `internal/web/static/app.css`. Assets are vendored, not loaded from a CDN.
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
| `COOKIE_SECURE` | `true` | Set `false` for local http development. Chrome and Firefox accept a `Secure` cookie from `http://localhost`; Safari drops it silently, and no browser accepts it from `http://192.168.x.x`, so testing from a phone needs this too. |
| `TELEGRAM_OAUTH_BASE_URL` | `https://oauth.telegram.org` | OIDC provider base. Point it at a login broker for local development. |
| `TELEGRAM_EXPECTED_ISSUER` | `https://oauth.telegram.org` | The exact `iss` the id_token must carry, compared byte for byte. Changed together with the base URL and never derived from it: pointing the flow somewhere else has to be deliberate. |

The web UI is optional: with `APP_BASE_URL`, `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET` or `JWT_SECRET` unset it is not mounted and only the bot runs. A warning is logged at startup saying which variables are missing.

### Local development without ngrok

Telegram only accepts publicly resolvable HTTPS redirect URIs, which is why the
login used to be untestable locally. [telegram-login-broker](../telegram-login-broker)
solves that: it is registered in BotFather once and forwards the browser to any
loopback port afterwards. Run it in mock mode and point two variables at it:

```sh
TELEGRAM_OAUTH_BASE_URL=http://127.0.0.1:8091
TELEGRAM_EXPECTED_ISSUER=http://127.0.0.1:8091
TELEGRAM_CLIENT_ID=makeme
TELEGRAM_CLIENT_SECRET=dev-secret     # must match the broker's BROKER_CLIENTS
COOKIE_SECURE=false
```

The whole exchange can be exercised without starting the bot at all:

```sh
BROKER_URL=http://127.0.0.1:8091 BROKER_CLIENT_ID=makeme BROKER_CLIENT_SECRET=dev-secret \
  go test ./internal/web -run TestLoginAgainstALiveBroker -v
```

That test drives this package's own `OIDCClient` — not a stub — through a real
broker: authorization URL, PKCE, code exchange, id_token parsing, and a replay of
the spent code. Without `BROKER_URL` it skips, so `go test ./...` stays hermetic.

What is already proven and what still has to be checked against real Telegram is
tracked in the broker's [docs/VERIFICATION.md](../telegram-login-broker/docs/VERIFICATION.md).

⚠️ **Starting the binary needs a separate dev bot.** `main.go` calls
`SetWebhook(ctx, cfg.WEBHOOK_URL)` on startup, and with `WEBHOOK_URL` empty that
**deletes the webhook** of whichever bot's token is in `TELEGRAM_TOKEN`. Running
locally with the production token takes production down.

### First deployment of the web UI

Not yet performed. The OIDC half of the login now works end to end against a real
independent implementation (see above), but the interface has never run against
Telegram itself, and no browser has yet been asked to accept its cookies. See
"Веб-интерфейс: что не проверено" in `docs/plan.md` for what that leaves unproven.

#### Where the image comes from

The target is krill, which deploys ready-made images rather than building from
source. CI publishes one on every push to `master`, after the tests pass:

```
ghcr.io/proshik/makeme:<commit-sha>   # deploy this one — it pins the exact build
ghcr.io/proshik/makeme:latest
```

The name is lower-case on purpose: GHCR rejects upper case, so it is not the
repository's `makeMe`. The image is `linux/amd64` only, listens on `8080`
(`EXPOSE 8080`, matching the `PORT` default), runs as uid 10001, and carries
`migrations/` and `data/` under `/app`. Migrations and the JSON seed run on every
start, so a fresh database needs nothing beyond the variables below. No image has
been published yet. The repository is public, and the package is meant to be
public as well, so krill pulls it anonymously: choose "Public (no auth)" for the
app's registry, no credential needed. Check the package's visibility in its
settings on GitHub after the first publish and switch it to public if needed —
until then an anonymous pull is refused.

#### Environment on krill

Values live in krill's secret store, never in the repository.

| Variable | Why it is required |
|---|---|
| `DATABASE_URL` | Startup fails without it. |
| `TELEGRAM_TOKEN` | Startup fails without it. The **production** bot. |
| `WEBHOOK_URL` | `https://<host>/webhook`. **Must be set:** empty deletes the bot's webhook on startup. |
| `APP_BASE_URL` | Public origin; the redirect URI is derived from it. |
| `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET` | OAuth pair from BotFather. Without them the web UI is not mounted. |
| `JWT_SECRET` | Session key, ≥32 bytes base64. Without it the web UI is not mounted. |
| `ADMIN_TOKEN` | Needed for `POST /admin/seed-audio` in step 5; without it every `/admin/*` returns 503. |

Leave `TELEGRAM_OAUTH_BASE_URL`, `TELEGRAM_EXPECTED_ISSUER` and `COOKIE_SECURE`
at their defaults in production: the first two point at Telegram itself, and the
cookie must stay `Secure`. `PORT`, `TIMEZONE`, `SCHEDULER_TIME` and the rest are
optional (see the table above).

#### Steps

1. Register an OAuth application for the bot and obtain `TELEGRAM_CLIENT_ID` / `TELEGRAM_CLIENT_SECRET`. These are **not** the same as `TELEGRAM_TOKEN`.
2. `openssl rand -base64 32` → `JWT_SECRET`. Startup fails if it is shorter than 32 bytes decoded, so a bad value surfaces immediately rather than on first login.
3. Set `APP_BASE_URL` to the public origin. The redirect URI is derived as `$APP_BASE_URL/auth/telegram/callback` and must match what was registered exactly.
4. Deploy `ghcr.io/proshik/makeme:<sha>` with the environment above. In the logs expect `migrations applied`, three `seed:` lines and `web UI enabled`; then confirm in this order — each step depends on the previous one working:
   - `GET /login` renders and the button leads to Telegram.
   - Signing in lands back on `/` as the right user. A user who already exists in the bot must keep their level and progress.
   - `websocat -H="Cookie: auth_token=<jwt>" wss://<host>/ws` stays open longer than the platform's idle timeout. The server pings every 25s; a shorter proxy timeout will cut the socket.
   - Answer a task in the bot with the browser open on `/leaderboard` — the standings should move without a reload.
5. **After the first deploy, run `POST /admin/seed-audio`** (see [Production workflow](#production-workflow)). `audio_url` is not in `data/words.json` and the startup seeder never fills it, so on a fresh database it is empty for every word. Until then the "Послушать" button has nothing to play, and that path has never been exercised.

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

| `GET /leaderboard` | Group standings, day or month. `?group=` is honoured only for groups the user belongs to — any other id falls back to their own, so an id cannot expose another group's standings. |
| `GET /profile` | Level, monthly score, streak, vocabulary split by how each word got there, per-CEFR-level progress, group list. |
| `GET /groups`, `POST /groups/create`, `POST /groups/join` | Create a group (returns an invite code) or join by code. A wrong code and a missing group give the same message, so codes cannot be probed. |

Topic names travel as query values rather than path segments: categories like `can/can't` carry slashes, which a path segment would mangle.

`/learn` and `/words` write no score rows — not contributing to the leaderboard is its one behavioural difference from `/tasks`, and a test guards it.
| `GET /static/*` | Self-hosted assets, served from the binary via `embed.FS`. |

Asset URLs carry a `?v=` token derived from a hash of every embedded file, so `Cache-Control: immutable` is safe — a new build produces new URLs. Without it a deploy would leave browsers on the previous stylesheet for a year.

Session cookies are `HttpOnly`, `Secure`, `SameSite=Lax` — Lax is the CSRF defence for cookie auth, which is why the POST handlers carry no separate CSRF token. The `id_token` signature is deliberately not verified: it arrives from Telegram's token endpoint over TLS server-to-server, and there is no public JWKS to check against; `exp`, `aud` and `iss` are validated as defence in depth.

### Live updates

`GET /ws` is a WebSocket carrying event names, not rendered HTML. A tab may be on any page, so the server cannot know what to swap; it says what changed and the client refetches through the ordinary handlers, keeping one rendering path for both the first load and the update.

Two events exist. `progress` goes to the user's own sessions when they answer anywhere. `leaderboard` goes to everyone in the group, and to the user themselves, but only when the answer actually scored — a wrong answer cannot move the standings.

What each page does with an event is a deliberate split. The leaderboard refetches itself (`hx-trigger="makeme:leaderboard from:body"`): it is a passive page with nothing to lose. Tasks and words only reveal a notice offering to reload — pulling the slot out from under someone who is typing an answer is worse than showing them something slightly stale.

Four things this needed that are easy to miss:

- `http.Server`'s `WriteTimeout` is the usual suspect for a socket that dies on a timer, and this server sets one — but it does not apply. `net/http` clears both deadlines when a connection is hijacked, which `websocket.Accept` does. `TestSocketOutlivesWriteTimeout` pins that down against a deliberately short timeout, so the assumption is rechecked if the value or the library ever changes.
- The hub cannot tell sessions apart, so the tab that answered also receives `progress`. Threading a socket identity through a domain event would drag transport into the domain; the client suppresses the notice for three seconds after its own request instead.
- The hub is required. `realtime.js` is on every page and always opens `/ws`, so a web UI without a hub is broken rather than degraded: the first production deploy shipped with the hub created but never passed into `web.Deps`, `/ws` answered 404 and every tab kept reconnecting, with nothing in the logs. `web.New` now returns `ErrNoHub` instead, and the binary refuses to start (`web: Deps.Hub is nil; the web UI needs the realtime hub`), so the same slip fails the deploy. With the web UI disabled no hub is needed.
- The hub is in-process, so events reach only sessions on this instance. The domain talks to `task.Notifier`, not to the hub, so a Postgres `LISTEN/NOTIFY` implementation can replace it without the domain noticing.

### Navigation

The header carries two levels, because it holds two different kinds of thing. On the left are places to work — Задания, Слова, Грамматика, Рейтинг — with the current one marked by an amber underline and `aria-current="page"`. On the right is the user: a chip with their name and level opening a menu with Профиль, Группы, appearance and Выйти.

That menu is a native `<details>`, not a scripted dropdown: `script-src 'self'` forbids inline handlers, and a disclosure element is keyboard-accessible with no JavaScript at all. `app.js` adds only what `<details>` lacks — closing on an outside click and on Escape.

Below 720px the account name is hidden (the level chip stays, since it carries information) and the sections scroll horizontally rather than wrapping. The breakpoint is measured, not guessed: at 606px the four sections overflowed by 20px while the name still occupied the chip.

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

Response: streamed `application/x-ndjson`, one line per event (start of level / per-word result / final summary). Use `curl -N` so output isn't buffered.

A run lasts minutes, far longer than the server's 10s `WriteTimeout`, which is a deadline on the whole response rather than on each line. For this response only, the handler replaces it with a 30s deadline on each line, re-armed right before every write; every other endpoint keeps the global one. The gap between lines does not count, so a slow dictionary lookup or a large `delay_ms` does not cut the stream. Without that the stream was cut after ~10s with a few lines delivered and no `done`. The per-line bound is what still releases a run whose client keeps the connection open but stops reading: the write fails, the handler returns and the request context stops the seeder. `TestSeedAudio_StreamOutlivesServerTimeouts`, `TestSeedAudio_StalledClientReleasesRun` and `TestSeedAudio_ClientDisconnectCancelsRun` run the handler behind a real `http.Server` with sub-second timeouts and the same middleware as production — the deadlines reach the connection only through the middleware's `Unwrap`, so a wrapper without it would turn every run into a 500. A reverse proxy in front of the app can still impose its own limit on response length; that has not been checked on production.

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

3. **If 429 appears** (`"status":"failed","error":"unexpected status: 429"`) — Free Dictionary throttled you. Wait 1–2 minutes and re-run with the same body; the run is idempotent so already-populated rows are skipped. Words that came back `not_found` have no `audio_url` either, so every re-run asks the API about them again and spends `delay_ms` on each.

4. **Adding more levels** later (B1–C2): same call, change `levels`. ~608 words across B1–C2, allow ~10 minutes total.

#### Limits to expect

* Free Dictionary returns no audio at all for ~3–5% of common words (e.g. `look`, `room`, `sea`); they end up as `"status":"not_found"` and stay with empty `audio_url`. The user-facing `🔊 Послушать` button will reply "Произношение не найдено" for these — acceptable for v1.
* Disconnect (closing curl, network drop) is handled: the server cancels the run via `ctx.Done()` so no quota is wasted on a dropped client. A run that ends without its `done` line, whether the client left, stopped reading or a proxy cut the response, is logged as `admin seed-audio aborted before done` with the cause, not as `completed`. Look for that line in the server log when a client saw no `done`.

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

### Docker image

```bash
docker build --platform linux/amd64 -t makeme:local .
```

The builder stage runs on the host's own platform and cross-compiles to the
target, so this works on Apple Silicon too. Running the Go toolchain itself under
qemu crashes in `go mod download`. `tailwind.css` is not rebuilt inside the image:
`//go:embed` takes the committed file, so run `make css` first if templates
changed. `.dockerignore` keeps `.env` out of the build context. The context is
copied wholesale, and without that rule real tokens would end up in an image
layer.

`docker compose up` builds the same image and runs it next to PostgreSQL. Use a
dev bot's token in `.env`; see the warning about `WEBHOOK_URL` above.

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

CI (`.github/workflows/ci.yml`) has three jobs:

- **test** — on every push and PR to `master`: `make css-check`, then `go test -v -race ./...` with the Go version from `go.mod`. The runner has Docker, so the testcontainers suites run there too.
- **docker-build** — on PRs only: builds the image for `linux/amd64` without pushing, so a broken Dockerfile shows up before the merge.
- **publish** — on push to `master` or a manual run on `master`, after `test`: pushes `ghcr.io/proshik/makeme:latest` and `:<sha>` with the workflow's `GITHUB_TOKEN` (`packages: write`).

The workflow can also be started by hand (`workflow_dispatch`: **Actions → CI → Run workflow**, or `gh workflow run ci.yml --ref master`). On `master` a manual run tests and publishes, just like a push; on any other branch it only runs `test`.

A newer push to a PR cancels the older run. On `master` a running publish is never interrupted and runs go one at a time, so `latest` is never overwritten by an older commit. GitHub keeps only the newest *pending* run, though, so in a burst of pushes an intermediate commit may get no image.
