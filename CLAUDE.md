# makeMe — English Learning Game

## Project Overview

A gamified English learning platform for Russian speakers. Users join groups and compete daily by completing English learning tasks. The system adapts to each user's skill level so players with different proficiency can compete fairly.

## Core Concepts

### Groups
- Users form groups (2+ people)
- Groups compete collectively and individually
- Daily, weekly, and monthly leaderboards per group

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

## Architecture Notes (TBD)

- Domain-driven design preferred — core domains: `user`, `group`, `task`, `scoring`, `assessment`, `words`
- Client: Telegram bot (webhook mode, integrated into chi router)
- REST API via chi serves `/health`, `/webhook`, and `/admin/*` (bearer-token gated, see Workflow Rules)
- Bulk operational logic (e.g. `internal/audio.Seed`) lives in its own package and is reused by both the HTTP admin handler and a `cmd/<tool>` CLI — never duplicate the logic between them

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
- **Words** (`data/words.json`) and **Grammar questions** (`data/grammar.json`) are seeded **automatically on every app startup** — no manual command needed.
- Migrations are for **schema changes only** (CREATE TABLE, ALTER TABLE, indexes).
- Adding or editing words/grammar questions = edit the JSON file and restart the app (or redeploy). Never put INSERT rows for these tables into a migration file.

#### Audio (`words.audio_url`) is NOT in JSON
- `data/words.json` does not carry `audio_url`. The startup seeder (`UpsertWord`) writes only english/russian/level/example/transcription, and never overwrites `audio_url`.
- Audio is fetched lazily from the Free Dictionary API: by users tapping the **🔊 Послушать** button, or in bulk via the admin endpoint / CLI.
- After deploying to a new environment (or after adding many new words), populate audio with `POST /admin/seed-audio` — see README "Production workflow" section. Same logic available offline as `go run ./cmd/seed-audio -levels=...`.
- Idempotent: re-running skips rows that already have `audio_url`. ~3–5% of common words have no audio in the API at all (`look`, `room`, `sea`, …) — that's expected; the user-facing button replies "Произношение не найдено" for those.

#### Admin endpoints
- All `/admin/*` endpoints are bearer-token protected via `ADMIN_TOKEN` env. With the env unset, every admin endpoint returns 503 — this is intentional, do not remove the check.
- Constant-time token compare via `crypto/subtle.ConstantTimeCompare` — keep that pattern when adding more admin endpoints.
- Don't tunnel admin actions through `/webhook` or other public paths. New admin operations belong under `r.Route("/admin", …)` in `internal/server/server.go`.

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

Phase 1 (skeleton & infra) complete. See `PLAN.md` for detailed progress.
