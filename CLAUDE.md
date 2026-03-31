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

- Domain-driven design preferred — core domains: `user`, `group`, `task`, `scoring`, `assessment`
- Client: Telegram bot (webhook mode, integrated into chi router)
- REST API via chi for internal/admin endpoints if needed

## Development Principles

- Keep domain logic separate from infrastructure
- Spaced repetition algorithm for vocabulary (SM-2 or similar)
- All user-facing text must support Russian localization
- Scoring normalization formula must be deterministic and auditable

## Workflow Rules

### Progress Tracking
- After completing any work, always update `PLAN.md` — mark completed items with `[x]` and phase with `[DONE]`
- If there is no plan file, mention this to the user and suggest creating one

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
