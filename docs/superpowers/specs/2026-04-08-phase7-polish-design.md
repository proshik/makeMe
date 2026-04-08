# Phase 7 — Polish Design

**Date:** 2026-04-08
**Status:** Approved

---

## Overview

Polish the product for daily use: `/profile` command with user statistics, graceful shutdown, Docker packaging, and documented `.env.example`.

Proactive messages (scheduler) are deferred to Phase 8.

---

## `/profile` Command

### Handler

New file: `internal/telegram/handler_profile.go`

Registered in `bot.go`:
```go
tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/profile", bot.MatchTypeExact, b.handleProfile)
```

### Message format

```
👤 Твой профиль

Уровень: B1
Баллы за месяц: 142.5 очков
Дней подряд: 5
Выучено слов: 47 (слово считается выученным после 3 повторений)
```

If the user is not onboarded (no level), show:
```
Сначала пройди оценку уровня: /assess
```

### Data sources

**Monthly score** — new sqlc query `GetUserMonthlyScore` in `queries/scoring.sql`:
```sql
-- name: GetUserMonthlyScore :one
SELECT COALESCE(SUM(normalized), 0)::float8 AS normalized
FROM scores
WHERE user_id = $1 AND period = 'month' AND period_key = $2;
```
Called with current month key `YYYY-MM`. Returns 0 if no scores yet.

**Streak** — new sqlc query `GetUserScoreDays` in `queries/scoring.sql`:
```sql
-- name: GetUserScoreDays :many
SELECT DISTINCT period_key
FROM scores
WHERE user_id = $1 AND period = 'day'
ORDER BY period_key DESC;
```
Go calculates streak: starting from today, count consecutive days backwards. A day counts if its `YYYY-MM-DD` string is present in the result set. Stop at the first gap.

Streak calculation in Go:
```go
func calcStreak(days []string) int {
    set := make(map[string]bool, len(days))
    for _, d := range days {
        set[d] = true
    }
    streak := 0
    for d := time.Now(); ; d = d.AddDate(0, 0, -1) {
        if !set[d.Format("2006-01-02")] {
            break
        }
        streak++
    }
    return streak
}
```

**Words learned** — new sqlc query `CountLearnedWords` in `queries/srs.sql`:
```sql
-- name: CountLearnedWords :one
SELECT COUNT(*)::int FROM srs_items
WHERE user_id = $1 AND repetitions >= 3;
```

### New repository methods

`ScoringRepository` interface (`internal/domain/scoring/repository.go`):
```go
GetMonthlyScore(ctx context.Context, userID int64, periodKey string) (float64, error)
GetScoreDays(ctx context.Context, userID int64) ([]string, error)
```

`SRSRepository` interface (`internal/domain/srs/repository.go`):
```go
CountLearnedWords(ctx context.Context, userID int64) (int, error)
```

Implementations in `internal/storage/postgres/scoring.go` and `internal/storage/postgres/srs.go`.

---

## Graceful Shutdown

`main.go` already has `signal.NotifyContext` and `srv.Shutdown`. Missing: waiting for the `StartWebhook` goroutine.

Change in `run()`:
```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    tgBot.StartWebhook(ctx)
}()

// ... start HTTP server ...

// After srv.ListenAndServe returns:
wg.Wait()
```

---

## Dockerfile

Multi-stage build. Migrations directory is copied into the image so `runMigrations` works at startup.

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /makeme ./cmd/makeme

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /makeme /makeme
COPY --from=builder /app/migrations /app/migrations
CMD ["/makeme"]
```

---

## docker-compose.yml

Two services: `postgres` with healthcheck, `app` with `depends_on: postgres: condition: service_healthy`.

```yaml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: makeme
      POSTGRES_USER: makeme
      POSTGRES_PASSWORD: makeme
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U makeme"]
      interval: 5s
      timeout: 5s
      retries: 5
    volumes:
      - pgdata:/var/lib/postgresql/data

  app:
    build: .
    env_file: .env
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  pgdata:
```

The `.env` file used by `app` overrides `DATABASE_URL` to point to the `postgres` service:
```
DATABASE_URL=postgres://makeme:makeme@postgres:5432/makeme?sslmode=disable
```

---

## `.env.example`

Add inline comments for every variable:

```bash
# HTTP server port
PORT=8080

# PostgreSQL connection string (pgx format)
DATABASE_URL=postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable

# Telegram bot token from @BotFather
TELEGRAM_TOKEN=your-telegram-bot-token

# Public HTTPS URL where Telegram will POST updates (must include /webhook path)
WEBHOOK_URL=https://your-domain.com/webhook

# Log level: debug | info | warn | error
LOG_LEVEL=info
```

---

## Testing

### Unit test — streak calculation

`internal/telegram/handler_profile_test.go` (or a helper package):
- `TestCalcStreak_ConsecutiveDays` — 5 days in a row → streak = 5
- `TestCalcStreak_GapYesterday` — today missing → streak = 0
- `TestCalcStreak_GapInMiddle` — days 1,2,4,5 (gap on day 3) → streak = 2 from today

### Integration test — `internal/telegram/handler_profile_test.go`

- `TestHandleProfile_NotOnboarded` — user without level → "пройди оценку" message
- `TestHandleProfile_Onboarded` — user with level, scores, srs_items → message contains level, score, streak, word count

---

## Out of scope

- Proactive messages / scheduler (Phase 8)
- Re-assessment with cooldown (Phase 8)
- Retry logic for Telegram API calls
