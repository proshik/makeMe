# makeMe — English Learning Game

Instructions for coding agents working in this repository. `CLAUDE.md` only
imports this file (`@AGENTS.md`), so edit the rules here.

## Project Overview

A gamified English learning platform for Russian speakers. Users join groups and compete daily by completing English learning tasks. The system adapts to each user's skill level so players with different proficiency can compete fairly.

There are two clients over one PostgreSQL state and the same domain services: a Telegram bot and a web UI with Telegram login. A user can start in one and continue in the other.

## Core Concepts

### Groups
- Users form groups (2+ people)
- Groups compete collectively and individually
- Daily, weekly, and monthly leaderboards per group — **target state**; only `day` and `month` are implemented (see Project Status)

### Daily Task System
Each day, every group member receives a set of tasks covering:
- **Vocabulary** — new words + spaced repetition of previously learned words
- **Grammar** — grammar exercises
- **Other task types** — listening, reading comprehension, translation, etc. (planned, not implemented)

Tasks are generated daily and differ each day.

### Scoring & Fairness
- Points are awarded per task; full points for zero mistakes
- Task difficulty is factored into scoring so players at different levels earn comparable points
- This allows fair competition within mixed-level groups
- Daily scores aggregate into monthly totals (weekly is not implemented)

### Initial Assessment
- New users complete an onboarding assessment to determine their base English level (A1–C2)
- Task difficulty and scoring coefficients are calibrated to this baseline
- Level can be re-evaluated over time as the user progresses

### Summaries
- End-of-day summary for each group (scheduler); end-of-week and end-of-month summaries are planned
- Individual progress tracking alongside group rankings

## Tech Stack

- **Language:** Go 1.26
- **Module:** `makeMe`
- **Database:** PostgreSQL
- **DB driver:** pgx
- **SQL codegen:** sqlc
- **Migrations:** goose
- **HTTP router:** chi
- **Telegram bot:** github.com/go-telegram/bot (inline keyboards are built in the handlers, no widget library)
- **Web UI:** `html/template` + htmx 2.0.4, Tailwind (pre-built), WebSocket via github.com/coder/websocket
- **Config:** environment variables + .env

## Architecture Notes

- Domain-driven design — core domains under `internal/domain/`: `user`, `group`, `task`, `scoring`, `assessment`, `srs`, `words`
- Supporting packages: `internal/textmatch` (fuzzy answer comparison for typing modes), `internal/dictionary` (Free Dictionary API client), `internal/audio` (bulk audio seeding), `internal/seed` (JSON seed on startup), `internal/scheduler` (daily summaries + reassessment reminders), `internal/realtime` (in-process WebSocket hub), `internal/config`, `internal/storage/{postgres,sqlc}`
- Two thin adapters over the same domain services: `internal/telegram` (bot, webhook mode) and `internal/web` (browser UI). A feature that users see usually has to be built in both.
- `internal/server` builds the chi router: `/health`, `/webhook`, `/admin/*` (bearer-token gated, see Workflow Rules), and mounts the web UI when it is configured (routes in `docs/web-ui.md`)
- Bulk operational logic (e.g. `internal/audio.Seed`) lives in its own package and is reused by both the HTTP admin handler and a `cmd/<tool>` CLI — never duplicate the logic between them
- **The CEFR ladder lives in one place: `user.AllLevels()`, `Level.Index/Next/Prev/AtLeast`.** It used to be restated in four files, so changing the ladder meant finding all of them. Don't write `[]user.Level{user.A1, ...}` again — a test asserts `AllLevels` returns a copy, so callers cannot reorder the canonical one either.
- Domain packages define repository interfaces; `internal/storage/postgres` implements them over sqlc-generated code. Never let `internal/domain/*` import sqlc types.
- **Task prompts are escaped once, for Telegram.** `internal/domain/task` stores `Task.Prompt` already escaped for Telegram's HTML parse mode (`html.EscapeString`; vocab prompts also carry `<i>` markup). Options, the answer and the rule are not escaped. The web must unescape the prompt before handing it to `html/template` (`html.UnescapeString` in `buildTaskView`), or the user sees `I&#39;ll`. Do not move the escaping into the template or add a second one; the bot depends on the escaped value.

### sqlc
- `make install-tools` installs sqlc (`@latest`) and goose into `~/go/bin` from the public Go proxy; `make sqlc` runs `~/go/bin/sqlc generate` over `queries/` and `migrations/` (`sqlc.yaml`). Never edit `internal/storage/sqlc` without updating `queries/` to match.
- The latest change to the generated code (`users.sql.go`, `models.go` for `show_rule_on_correct`, commit `83dc3ce`) was written by hand in the sqlc v1.30.0 format, because sqlc was not installed. `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 diff` reported no difference on 2026-11-28, so the first real regeneration should produce an empty diff; a newer sqlc may change only the version header. Anything else means the generated code drifted from `queries/` — fix it before building on top. The same `go run …@v1.30.0` form works without installing sqlc.

## Development Principles

- Keep domain logic separate from infrastructure
- Spaced repetition algorithm for vocabulary (SM-2)
- All user-facing text must support Russian localization
- Scoring normalization formula must be deterministic and auditable

## Workflow Rules

### Progress Tracking
- After completing any work, always update `docs/plan.md` — mark each completed item with `[x]` and phase header with `[DONE]`. Top of the file has a `## Реализованные фичи (changelog)` section — prepend new entries there.
- After completing any work, always update the docs: new commands and the typical flow diagram in `README.md`; the database schema in `docs/database.md`; env vars in `docs/configuration.md`; admin endpoints in `docs/admin.md`; web routes in `docs/web-ui.md`; local development, tests and CI in `docs/development.md`; production rollout in `docs/deployment.md`.
- This file holds rules for the agent, not history. History goes into the changelog in `docs/plan.md`.
- If there is no plan file, mention this to the user and suggest creating one.

### Seed Data — NEVER use migrations for content
- Three files are seeded **automatically on every app startup** — no manual command needed:
  - **Words** — `data/words.json` (env `WORDS_FILE`)
  - **Grammar questions** — `data/grammar.json` (env `GRAMMAR_QUESTIONS_FILE`)
  - **Grammar category theory** — `data/grammar_categories.json` (env `GRAMMAR_CATEGORIES_FILE`), the explanations `/learn` shows above each topic and both clients show as the rule after a grammar answer
- Migrations are for **schema changes only** (CREATE TABLE, ALTER TABLE, indexes).
- Adding or editing this content = edit the JSON file and restart the app (or redeploy). Never put INSERT rows for these tables into a migration file.
- Migrations `00006`, `00011`, `00014`, `00015`, `00017` predate this rule and still carry content INSERTs. Leave them alone (they are already applied everywhere) — but never add another one like them.
- A seed file that is missing or fails to parse is logged via `slog.Warn` and skipped; startup continues. Check the logs after a deploy if content looks stale.

#### Grammar theory format (`data/grammar_categories.json`)
The web UI draws each explanation in blocks: `parseRule` (`internal/web/rule.go`) reads the plain text by its conventions and the single `rule_doc` template (`internal/web/templates/fragments/rule.html`) renders the topic page and both feedbacks (`/tasks`, `/learn`). Keep new and edited text in this format:
- blocks separated by an empty line; the first block is the lead paragraph
- a heading line ending in `:` (`Форма:`, `Примеры:`)
- list items start with `• `
- examples as `English sentence. — Русский перевод.`
- mistakes as `✗ wrong → ✓ right (примечание)`
- captions as `Подпись: …`; a block may open with a captioned line instead of a heading (`Present Simple: don't / doesn't + V`)

A line that does not match a convention exactly is kept as an ordinary paragraph — nothing is lost, but it is no longer drawn as an example, mistake or heading. `TestParseRuleKeepsEveryShippedExplanation` runs over every shipped topic: it fails if a line is lost or changed by the parse, or if a line starting with `✗` is not recognised as a mistake. A malformed example or heading does **not** fail it, so open the topic page after editing. Every part stays a plain string escaped by `html/template`; never turn data into `template.HTML`. The bot ignores the format and sends the text as is.

#### Audio (`words.audio_url`) is NOT in JSON
- `data/words.json` does not carry `audio_url`. The startup seeder (`UpsertWord`) writes only english/russian/level/example/transcription, and never overwrites `audio_url`.
- The bot fetches audio lazily from the Free Dictionary API when a user taps **🔊 Послушать** and caches it in `words`; bulk filling is the admin endpoint / CLI. The web UI never calls the API: it plays whatever `audio_url` the database already holds.
- After deploying to a new environment (or after adding many new words), populate audio with `POST /admin/seed-audio` — see "Production workflow" in `docs/admin.md`. Same logic available offline as `go run ./cmd/seed-audio -levels=...`.
- Idempotent: re-running skips rows that already have `audio_url`. ~3–5% of common words have no audio in the API at all (`look`, `room`, `sea`, …) — that's expected; for those the bot replies «Произношение для «word» не найдено.».
- The mp3 links the API returns answered 404 when checked with curl on 2026-11-27 (`shirt`, `hello`, `cat`); playback in a browser was not checked. The bot has no fallback for a dead link. The web «Послушать» (no emoji) tries `data-src` first and, on any playback error or with no `audio_url`, speaks the word with the browser's speech synthesis (`en-US`, text from `data-text`); «Не воспроизводится» appears only when neither works.
- In the web UI the listen button must not appear before the answer where hearing the word gives it away: RU→EN modes in `/words`, RU→EN and fill-in-the-blank tasks in `/tasks`. The `/words` feedback card offers it in every mode. The bot offers it only in the feedback, after the answer.

#### Admin endpoints
- All `/admin/*` endpoints are bearer-token protected via `ADMIN_TOKEN` env. With the env unset, every admin endpoint returns 503 — this is intentional, do not remove the check.
- Constant-time token compare via `crypto/subtle.ConstantTimeCompare` — keep that pattern when adding more admin endpoints.
- Don't tunnel admin actions through `/webhook` or other public paths. New admin operations belong under `r.Route("/admin", …)` in `internal/server/server.go`.

### Web UI assets
- Nothing on a page may come from a third-party origin. Everything is served from `internal/web/static/` via `embed.FS`, which is what lets the CSP stay at `default-src 'self'`. Two tests enforce this — don't work around them by relaxing the CSP. The one deliberate exception is `media-src 'self' https:`, for pronunciation mp3s served straight from the Free Dictionary API.
- **`script-src 'self'` has no `'unsafe-inline'`, and that constrains the templates:** no inline `<script>`, no `on*=""` handlers, no htmx `hx-on` / `js:` expressions. Put behaviour in `internal/web/static/js/app.js` and trigger it from a `data-action` attribute. Two tests fail on inline scripts or handlers. `style-src` does keep `'unsafe-inline'` — inline `style=""` for design tokens is an accepted trade-off, style injection being far less dangerous than script execution.
- **Tailwind is pre-built, never the browser compiler.** `cdn.tailwindcss.com` is the Play CDN and is development-only by Tailwind's own docs. Run `make css` (standalone CLI, no Node) after editing anything under `internal/web/templates/`, and commit the regenerated `internal/web/static/tailwind.css` — `//go:embed` needs it present at compile time. CI runs `make css-check` and fails on drift.
- `tailwind.config.js` zeroes `borderRadius` and `boxShadow` at the theme level, not via `extend`, so radius/shadow utilities don't exist. That's deliberate: the «Табло» design system forbids them. Don't re-enable.
- Design system lives in `internal/web/static/app.css` — that file is the source of truth, and its comments carry the reasoning. No emoji in the web UI — states are carried by colour, weight and plate. The Telegram bot keeps its emoji, since there is no CSS there.

### Web UI behaviour that is load-bearing
- **The next slot travels with the answer.** The `/tasks` and `/words` feedback fragments carry the next slot in an inert `<template data-next-slot>`; «Дальше» swaps it in from `app.js` without a request, and the button's `hx-get` stays as the fallback. The `/tasks` template is the same `task_slot` `/tasks/next` renders — options only, never the answer or the rule — and the last task of a round carries none. `/words` gets its next drill from `words.Service.PeekNextSlot`, which is `NextSlot` without its one write: it must **not** close the session, because closing behind the user's back would end the session before they press «Дальше»; `GET /words/next` still closes an exhausted session. No template when the card offers «Отложить до завтра», since deferring changes what comes next. A prefetched task answered in the bot meanwhile returns `HX-Refresh`. Tests: `internal/web/integration_prefetch_test.go`.
- **Rule after a grammar answer.** After a mistake or a skip the rule is always shown, in `/tasks` and in `/learn`. After a correct answer it is shown only when `users.show_rule_on_correct` is on (profile setting, `POST /profile/settings`, values `1`/`0`, default off). The bot does not read the setting.
- `/learn` and `/words` write no score rows; only `/tasks` counts toward the leaderboard.

#### Grammar `subcategory` values
Each grammar question carries a `subcategory` used by `/learn` to stratify the per-session sample so one practice covers different aspects of a rule. Allowed values:
- `affirmative` — plain positive sentence (e.g. Past Simple `She played tennis.`)
- `negative` — answer contains a negative contraction (`don't`, `didn't`, `won't`, `can't`, …)
- `interrogative` — prompt ends with `?`
- `spelling` — 3rd-person-singular / plural / ending spelling drill
- `general` — catch-all for rules where the affirmative/negative/interrogative split doesn't apply (Articles, Prepositions, Object pronouns, …)

Pick one of the above when adding new entries. Unknown values fall back to `"general"` automatically.

### Git & Commits
- Always suggest committing after completing a logical unit of work (phase, feature, fix)
- If git is not initialized — remind the user and offer to initialize
- Commit messages in English, concise, imperative mood

### Testing Strategy
- **Always write tests.** No feature is complete without tests.
- **Integration tests (TestContainers):** for API endpoints, Telegram bot commands (`internal/telegram`, PostgreSQL + WireMock) and web handlers (`internal/web`, PostgreSQL). TestContainers spins up a real PostgreSQL instance. These tests cover the full stack including the DAO layer, so separate DAO-only tests are unnecessary.
- **Unit tests:** for pure algorithms (SM-2, scoring normalization), validation logic, and utilities — anything that doesn't need a database or external services.
- Test files live next to the code they test (`*_test.go`).
- With colima instead of Docker Desktop, set `DOCKER_HOST` and `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE` as shown in `docs/development.md`; without them the testcontainers suites fail with "rootless Docker not found".

## Project Status

Phases 1–8.5, 10, 10.5 and 10.6 are done; phase 9 is partial (EN→RU, RU→EN, fill-in-the-blank and the typing modes exist; definition, spelling and context tasks do not). See `docs/plan.md` — the changelog at the top of that file is the authoritative feature list.

Telegram bot, shipped and working end to end:
- `/start`, `/help`, `/assess` (30 adaptive questions: 1–15 vocabulary, 16–30 grammar)
- `/newgroup`, `/join`, `/mygroups`, `/leaderboard` (day + month only — see below), `/profile`
- `/tasks` — 5 vocab + 5 grammar per round, up to `MAX_ROUNDS_PER_DAY` rounds, solo group auto-created
- `/words` — active-recall mode, 10 words per session, 4 modes × 2 correct answers, strictly ascending A1→C2 selection
- `/learn` — grammar theory + up to 20 stratified questions per topic
- Scheduler (daily summary at `SCHEDULER_TIME`, reassessment reminder after 30 days), `POST /admin/seed-audio`

Web UI (`internal/web`, in production since 2026-11-20), the same features over the same state:
- Telegram login (OAuth 2.0 + PKCE), `/tasks`, `/assess`, `/words` (auto-advance after the answer), `/learn`, `/leaderboard` (day/month, live), `/profile` with settings, `/groups`
- Grammar rules drawn in blocks, listening with a speech-synthesis fallback, the next slot sent along with the answer
- Live updates over `/ws` from an in-process hub (one replica; see `docs/web-ui.md`)

Web UI login (`internal/web`) — rules that are load-bearing, not preferences:
- The `id_token` signature is **not** verified (it arrives server-to-server over
  TLS), so the claim checks are the only thing between a token and an identity.
  Every one of `iss`, `aud`, `exp` is **mandatory**; `iat` is checked when present
  with 60s of skew. Never weaken one back to "checked only if present" — that
  hands the decision to whoever issued the token.
- `iss` is compared **byte for byte** against `TELEGRAM_EXPECTED_ISSUER`, never by
  substring. It is configured separately from `TELEGRAM_OAUTH_BASE_URL` so that
  pointing the flow at another OIDC provider is deliberate rather than incidental.
- `aud` is read both as a string and as an array. As a bare `.(string)` an array
  yields "" and the audience check turns itself off silently.
- A missing `id` claim is a failure. **Never fall back to `sub`** — it is a
  different number, and substituting it would return a stable, plausible and
  wrong person.
- The token HTTP client refuses redirects. There is no legitimate redirect on a
  token endpoint, and following one reads the id_token from wherever the last hop
  pointed.
- Telegram OAuth redirects only to the registered domain, so the web login is
  checked live only in production; locally it is covered by the `internal/web`
  tests against a fake OIDC provider (see `docs/development.md`). Start the
  binary locally with `TELEGRAM_LOCAL_MODE=true` and a **separate dev bot**. An empty `WEBHOOK_URL` without local mode is a startup
  error, and `Bot.SetWebhook` refuses an empty URL: Telegram reads it as
  deleteWebhook, which used to take down whichever bot's token was configured.
  Keep both guards.
- `/webhook` requires `X-Telegram-Bot-Api-Secret-Token` to equal
  `TELEGRAM_WEBHOOK_SECRET` (constant-time, 401 otherwise, 503 with no secret
  configured). The secret is mandatory while `WEBHOOK_URL` is set. Don't switch
  to go-telegram/bot's `WithWebhookSecretToken`: it compares with `!=` and
  answers a mismatch with 200.

Known gaps worth knowing before you touch these areas:
- **No weekly period exists.** `scoring.Service.Award` only upserts rows under the `"day"` and `"month"` period keys, and `/leaderboard` is registered `MatchTypeExact` — it accepts no text arguments. Periods switch via a single inline toggle button. Adding a week means new period keys plus new UI, not just a label.
- **The realtime hub is in-process.** Events reach only sessions on the same instance; with more than one replica they would be lost. The domain talks to `task.Notifier`, so a Postgres `LISTEN/NOTIFY` implementation can replace the hub without domain changes.

Not yet started (see plan phases 11–17): translation tasks, streak rewards, weekly/monthly group summaries, automatic level re-evaluation, a larger B2–C2 grammar bank, listening tasks, personal word lists. These phases are written for the bot; each now has to be built in the web UI too.
