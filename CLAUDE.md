# makeMe — English Learning Game

## Project Overview

A gamified English learning platform for Russian speakers. Users join groups and compete daily by completing English learning tasks. The system adapts to each user's skill level so players with different proficiency can compete fairly.

## Core Concepts

### Groups
- Users form groups (2+ people)
- Groups compete collectively and individually
- Daily, weekly, and monthly leaderboards per group — **target state**; only `day` and `month` are implemented (see Project Status)

### Daily Task System
Each day, every group member receives a set of tasks covering:
- **Vocabulary** — new words + spaced repetition of previously learned words
- **Grammar** — grammar exercises
- **Other task types** — listening, reading comprehension, translation, etc.

Tasks are generated daily and differ each day.

### Scoring & Fairness
- Points are awarded per task; full points for zero mistakes
- Task difficulty is factored into scoring so players at different levels earn comparable points
- This allows fair competition within mixed-level groups
- Daily scores aggregate into weekly and monthly totals

### Initial Assessment
- New users complete an onboarding assessment to determine their base English level (A1–C2)
- Task difficulty and scoring coefficients are calibrated to this baseline
- Level can be re-evaluated over time as the user progresses

### Summaries
- End-of-day, end-of-week, end-of-month summaries for each group
- Individual progress tracking alongside group rankings

## Tech Stack

- **Language:** Go 1.26
- **Module:** `makeMe`
- **Database:** PostgreSQL
- **DB driver:** pgx
- **SQL codegen:** sqlc
- **Migrations:** goose
- **HTTP router:** chi
- **Telegram bot:** github.com/go-telegram/bot (+ github.com/go-telegram/ui for widgets)
- **Config:** environment variables + .env

## Architecture Notes

- Domain-driven design — core domains under `internal/domain/`: `user`, `group`, `task`, `scoring`, `assessment`, `srs`, `words`
- Supporting packages: `internal/textmatch` (fuzzy answer comparison for typing modes), `internal/dictionary` (Free Dictionary API client), `internal/audio` (bulk audio seeding), `internal/scheduler` (daily summaries + reassessment reminders), `internal/storage/{postgres,sqlc}`, `internal/telegram`, `internal/server`
- Client: Telegram bot (webhook mode, integrated into chi router)
- REST API via chi serves `/health`, `/webhook`, and `/admin/*` (bearer-token gated, see Workflow Rules)
- Bulk operational logic (e.g. `internal/audio.Seed`) lives in its own package and is reused by both the HTTP admin handler and a `cmd/<tool>` CLI — never duplicate the logic between them
- **The CEFR ladder lives in one place: `user.AllLevels()`, `Level.Index/Next/Prev/AtLeast`.** It used to be restated in four files, so changing the ladder meant finding all of them. Don't write `[]user.Level{user.A1, ...}` again — a test asserts `AllLevels` returns a copy, so callers cannot reorder the canonical one either.
- Domain packages define repository interfaces; `internal/storage/postgres` implements them over sqlc-generated code. Never let `internal/domain/*` import sqlc types.

## Development Principles

- Keep domain logic separate from infrastructure
- Spaced repetition algorithm for vocabulary (SM-2 or similar)
- All user-facing text must support Russian localization
- Scoring normalization formula must be deterministic and auditable

## Workflow Rules

### Progress Tracking
- After completing any work, always update `docs/plan.md` — mark each completed item with `[x]` and phase header with `[DONE]`. Top of the file has a `## Реализованные фичи (changelog)` section — prepend new entries there.
- After completing any work, always update `README.md` — add new commands, update the database schema, update the typical flow diagram, document any new env vars or admin endpoints.
- If there is no plan file, mention this to the user and suggest creating one.

### Seed Data — NEVER use migrations for content
- Three files are seeded **automatically on every app startup** — no manual command needed:
  - **Words** — `data/words.json` (env `WORDS_FILE`)
  - **Grammar questions** — `data/grammar.json` (env `GRAMMAR_QUESTIONS_FILE`)
  - **Grammar category theory** — `data/grammar_categories.json` (env `GRAMMAR_CATEGORIES_FILE`), the explanations `/learn` shows above each topic
- Migrations are for **schema changes only** (CREATE TABLE, ALTER TABLE, indexes).
- Adding or editing this content = edit the JSON file and restart the app (or redeploy). Never put INSERT rows for these tables into a migration file.
- Migrations `00006`, `00011`, `00014`, `00015`, `00017` predate this rule and still carry content INSERTs. Leave them alone (they are already applied everywhere) — but never add another one like them.
- A seed file that is missing or fails to parse is logged via `slog.Warn` and skipped; startup continues. Check the logs after a deploy if content looks stale.

#### Audio (`words.audio_url`) is NOT in JSON
- `data/words.json` does not carry `audio_url`. The startup seeder (`UpsertWord`) writes only english/russian/level/example/transcription, and never overwrites `audio_url`.
- Audio is fetched lazily from the Free Dictionary API: by users tapping the **🔊 Послушать** button, or in bulk via the admin endpoint / CLI.
- After deploying to a new environment (or after adding many new words), populate audio with `POST /admin/seed-audio` — see README "Production workflow" section. Same logic available offline as `go run ./cmd/seed-audio -levels=...`.
- Idempotent: re-running skips rows that already have `audio_url`. ~3–5% of common words have no audio in the API at all (`look`, `room`, `sea`, …) — that's expected; the user-facing button replies "Произношение не найдено" for those.

#### Admin endpoints
- All `/admin/*` endpoints are bearer-token protected via `ADMIN_TOKEN` env. With the env unset, every admin endpoint returns 503 — this is intentional, do not remove the check.
- Constant-time token compare via `crypto/subtle.ConstantTimeCompare` — keep that pattern when adding more admin endpoints.
- Don't tunnel admin actions through `/webhook` or other public paths. New admin operations belong under `r.Route("/admin", …)` in `internal/server/server.go`.

### Web UI assets
- Nothing on a page may come from a third-party origin. Everything is served from `internal/web/static/` via `embed.FS`, which is what lets the CSP stay at `default-src 'self'`. Two tests enforce this — don't work around them by relaxing the CSP.
- **`script-src 'self'` has no `'unsafe-inline'`, and that constrains the templates:** no inline `<script>`, no `on*=""` handlers, no htmx `hx-on` / `js:` expressions. Put behaviour in `internal/web/static/js/app.js` and trigger it from a `data-action` attribute. Two tests fail on inline scripts or handlers. `style-src` does keep `'unsafe-inline'` — inline `style=""` for design tokens is an accepted trade-off, style injection being far less dangerous than script execution.
- **Tailwind is pre-built, never the browser compiler.** `cdn.tailwindcss.com` is the Play CDN and is development-only by Tailwind's own docs. Run `make css` (standalone CLI, no Node) after editing anything under `internal/web/templates/`, and commit the regenerated `internal/web/static/tailwind.css` — `//go:embed` needs it present at compile time. CI runs `make css-check` and fails on drift.
- `tailwind.config.js` zeroes `borderRadius` and `boxShadow` at the theme level, not via `extend`, so radius/shadow utilities don't exist. That's deliberate: the «Табло» design system forbids them. Don't re-enable.
- Design system lives in `internal/web/static/app.css` — that file is the source of truth, and its comments carry the reasoning. No emoji in the web UI — states are carried by colour, weight and plate. The Telegram bot keeps its emoji, since there is no CSS there.

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
- **Integration tests (TestContainers):** for API endpoints and Telegram bot commands. TestContainers spins up a real PostgreSQL instance. These tests cover the full stack including the DAO layer, so separate DAO-only tests are unnecessary.
- **Unit tests:** for pure algorithms (SM-2, scoring normalization), validation logic, and utilities — anything that doesn't need a database or external services.
- Test files live next to the code they test (`*_test.go`).

## Project Status

Phases 1–10.6 are complete — see `docs/plan.md` (the changelog at the top of that file is the authoritative feature list).

Shipped and working end to end:
- `/start`, `/help`, `/assess` (30 adaptive questions: 1–15 vocabulary, 16–30 grammar)
- `/newgroup`, `/join`, `/mygroups`, `/leaderboard` (day + month only — see below), `/profile`
- `/tasks` — 5 vocab + 5 grammar per round, up to `MAX_ROUNDS_PER_DAY` rounds, solo group auto-created
- `/words` — active-recall mode, 10 words per session, 4 modes × 2 correct answers, strictly ascending A1→C2 selection
- `/learn` — grammar theory + up to 20 stratified questions per topic
- Scheduler (daily summary at `SCHEDULER_TIME`, reassessment reminder after 30 days), `POST /admin/seed-audio`

Web UI login (`internal/web`) — rules that are load-bearing, not preferences:
- The `id_token` signature is **not** verified (it arrives server-to-server over
  TLS), so the claim checks are the only thing between a token and an identity.
  Every one of `iss`, `aud`, `exp` is **mandatory**; `iat` is checked when present
  with 60s of skew. Never weaken one back to "checked only if present" — that
  hands the decision to whoever issued the token.
- `iss` is compared **byte for byte** against `TELEGRAM_EXPECTED_ISSUER`, never by
  substring. It is configured separately from `TELEGRAM_OAUTH_BASE_URL` so that
  pointing the flow at a login broker is deliberate rather than incidental.
- `aud` is read both as a string and as an array. As a bare `.(string)` an array
  yields "" and the audience check turns itself off silently.
- A missing `id` claim is a failure. **Never fall back to `sub`** — it is a
  different number, and substituting it would return a stable, plausible and
  wrong person.
- The token HTTP client refuses redirects. There is no legitimate redirect on a
  token endpoint, and following one reads the id_token from wherever the last hop
  pointed.
- Local development runs against [telegram-login-broker](../telegram-login-broker);
  see the README. Starting the binary locally needs a **separate dev bot**:
  `SetWebhook` with an empty `WEBHOOK_URL` deletes the webhook of whichever bot's
  token is in `TELEGRAM_TOKEN`.

Known gaps worth knowing before you touch these areas:
- **No weekly period exists.** `scoring.Service.Award` only upserts rows under the `"day"` and `"month"` period keys, and `/leaderboard` is registered `MatchTypeExact` — it accepts no text arguments. Periods switch via a single inline toggle button. Adding a week means new period keys plus new UI, not just a label.

Not yet started (see plan phases 11–17): translation tasks, streak rewards, weekly/monthly group summaries, automatic level re-evaluation, listening tasks, personal word lists.
