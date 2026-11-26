# makeMe

Gamified English learning platform for Russian speakers: a Telegram bot and a web UI with Telegram login, sharing one PostgreSQL state. Users join groups and compete daily by completing English tasks adapted to their level.

## Screenshots

The web UI on demo data, in its dark and light themes (the image follows your GitHub theme).

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/tasks-question-dark.png">
  <img src="docs/images/tasks-question-light.png" alt="Daily task round: a grammar question with four options, task 6 of 10, level scale A1–C2 with B2 lit">
</picture>

<table>
  <tr>
    <td width="50%">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/tasks-feedback-dark.png">
  <img src="docs/images/tasks-feedback-light.png" alt="Feedback after a wrong answer: the correct option and the grammar rule behind it">
</picture>

<sub>Wrong answer: the correct option and the rule</sub>
    </td>
    <td width="50%">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/words-drill-dark.png">
  <img src="docs/images/words-drill-light.png" alt="Words drill: session counters, progress by CEFR level, per-mode tallies and a multiple-choice card">
</picture>

<sub>Words: active recall in four modes, progress by level</sub>
    </td>
  </tr>
  <tr>
    <td width="50%">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/assessment-question-dark.png">
  <img src="docs/images/assessment-question-light.png" alt="Level test: question 18 of 30, grammar phase">
</picture>

<sub>Level test: 30 adaptive questions</sub>
    </td>
    <td width="50%">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/assessment-result-dark.png">
  <img src="docs/images/assessment-result-light.png" alt="Level test result: B2, 23 of 30 correct">
</picture>

<sub>Test result with the level and the score</sub>
    </td>
  </tr>
  <tr>
    <td width="50%">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/leaderboard-dark.png">
  <img src="docs/images/leaderboard-light.png" alt="Group leaderboard for today: three players with normalized scores">
</picture>

<sub>Group leaderboard, normalized by level</sub>
    </td>
    <td width="50%">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/home-dark.png">
  <img src="docs/images/home-light.png" alt="Home page after sign-in: greeting and the current level">
</picture>

<sub>Home after sign-in</sub>
    </td>
  </tr>
</table>

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

## Quick start

The app needs PostgreSQL and the token of a **separate dev bot**: startup calls
Telegram's `getMe`, so a placeholder token is not enough, and the scheduler and
replies send real messages through whichever token is configured.

```sh
cp .env.example .env
# TELEGRAM_TOKEN       a dev bot from @BotFather, never the production one
# TELEGRAM_LOCAL_MODE  true, with WEBHOOK_URL empty: the bot's webhook is left alone
# DATABASE_URL         postgres://makeme:makeme@postgres:5432/makeme?sslmode=disable for compose

docker compose up --build        # app + PostgreSQL; migrations and seed data run on startup
curl http://localhost:8080/health
```

Without Docker, point `DATABASE_URL` at a local PostgreSQL and run `make run`.

The web UI is mounted only when `APP_BASE_URL`, `TELEGRAM_CLIENT_ID`,
`TELEGRAM_CLIENT_SECRET` and `JWT_SECRET` are set. To sign in locally without
real Telegram, run [telegram-login-broker](../telegram-login-broker) in mock mode
and point the OAuth variables at it, as described in
[docs/development.md](docs/development.md#local-development-without-ngrok).

## Documentation

| Document | Contents |
|---|---|
| [docs/configuration.md](docs/configuration.md) | Environment variables |
| [docs/development.md](docs/development.md) | `make` targets, local development with the login broker, Docker image, seed data, tests and CI |
| [docs/web-ui.md](docs/web-ui.md) | Web UI routes, live updates over `/ws`, navigation, self-hosted assets and the CSP |
| [docs/database.md](docs/database.md) | Database schema and relations |
| [docs/admin.md](docs/admin.md) | `/admin/*` endpoints and the audio seeding workflow |
| [docs/deployment.md](docs/deployment.md) | Production on krill: image, environment, webhook secret, checklist for a new environment |
| [docs/plan.md](docs/plan.md) | Roadmap and changelog (in Russian) |

## Tech Stack

Go, PostgreSQL (pgx/sqlc/goose), chi, [go-telegram/bot](https://github.com/go-telegram/bot)

## License

MIT — see [LICENSE](LICENSE).
