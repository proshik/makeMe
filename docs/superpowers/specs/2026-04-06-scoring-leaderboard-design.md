# Phase 6 — Scoring & Leaderboards Design

**Date:** 2026-04-06
**Status:** Approved

---

## Overview

Add a scoring system with per-task normalized points and group leaderboards (daily and monthly).
The Telegram bot gains a `/leaderboard` command showing all group members ranked by score.

---

## Database

### Migration `00009_create_scoring.sql`

```sql
CREATE TABLE scores (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    group_id    BIGINT NOT NULL REFERENCES groups(id),
    period      TEXT NOT NULL CHECK (period IN ('day', 'month')),
    period_key  TEXT NOT NULL,  -- 'YYYY-MM-DD' for day, 'YYYY-MM' for month
    raw         INT NOT NULL DEFAULT 0,
    normalized  NUMERIC(10,2) NOT NULL DEFAULT 0,
    UNIQUE (user_id, group_id, period, period_key)
);
```

One record per `(user_id, group_id, period, period_key)`.
On each correct answer: `INSERT ... ON CONFLICT DO UPDATE SET raw = raw + $delta, normalized = normalized + $norm_delta`.
Both `day` and `month` records are upserted in a single `RecordScore` call.

---

## Domain: `internal/domain/scoring/`

### `entity.go`

```go
type Score struct {
    ID        int64
    UserID    int64
    GroupID   int64
    Period    string  // "day" | "month"
    PeriodKey string  // "2026-04-06" | "2026-04"
    Raw       int
    Normalized float64
}

type LeaderboardEntry struct {
    UserID     int64
    FirstName  string
    Raw        int
    Normalized float64
    Rank       int
}
```

### `repository.go`

```go
type Repository interface {
    Upsert(ctx context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error
    GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]LeaderboardEntry, error)
}
```

### `service.go`

**Normalization coefficients:**

| Level | Coefficient |
|-------|-------------|
| A1    | 0.6         |
| A2    | 0.8         |
| B1    | 1.0         |
| B2    | 1.15        |
| C1    | 1.3         |
| C2    | 1.5         |

**Methods:**

- `RecordScore(ctx, userID, groupID int64, taskLevel user.Level, rawPoints int) error`
  Computes `normalized = rawPoints × coefficient(taskLevel)`, then upserts both `day` (key: `YYYY-MM-DD`) and `month` (key: `YYYY-MM`) records.

- `GetLeaderboard(ctx, groupID int64, period, periodKey string) ([]LeaderboardEntry, error)`
  Returns all group members sorted by `normalized DESC`, with `Rank` starting at 1.

---

## Task domain change

`Task` struct needs a `GroupID int64` field so `handleTaskCallback` can pass it to `RecordScore`
without an extra DB lookup. `task.Service.GetTaskByID` populates it via `DailySet`.

---

## Telegram

### `handler_task.go` — after SubmitAnswer

Only on correct answers:
```go
if result.Correct {
    b.scoringSvc.RecordScore(ctx, u.ID, task.GroupID, task.Level, result.Score)
}
```

### `handler_leaderboard.go` — new file

**`/leaderboard` command flow:**
1. Get user's groups.
2. If one group → immediately show today's leaderboard for that group.
3. If multiple groups → send inline keyboard: one button per group name.

**Callback format:** `lb:{groupID}:{period}:{periodKey}`
- Period: `day` or `month`
- Period key: `2026-04-06` or `2026-04`

**Leaderboard message format:**
```
🏆 Группа «Study» — сегодня

1. Alice — 23.0 очков
2. Bob — 18.4 очков
3. Charlie — 12.0 очков

[📅 За месяц]
```
The «За месяц» button switches to the monthly view (same group, `period=month`).
In monthly view, the button becomes «📅 За сегодня» to switch back.

### `bot.go` changes

- Add `scoringSvc *scoring.Service` to `Bot` struct
- Register `/leaderboard` → `handleLeaderboard`
- Register `lb:` prefix callback → `handleLeaderboardCallback`

---

## Storage: `internal/storage/postgres/scoring.go`

Implements `scoring.Repository` via sqlc. Queries in `queries/scoring.sql`:
- `UpsertScore` — insert or increment raw/normalized
- `GetLeaderboard` — starts from `memberships`, LEFT JOIN `scores` so members with no score appear with 0, JOIN `users` for first_name, filter by group+period+period_key, order by normalized DESC

```sql
SELECT u.id, u.first_name,
       COALESCE(s.raw, 0) AS raw,
       COALESCE(s.normalized, 0) AS normalized
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN scores s ON s.user_id = m.user_id
    AND s.group_id = $1
    AND s.period = $2
    AND s.period_key = $3
WHERE m.group_id = $1
ORDER BY normalized DESC;
```

---

## Testing

### Unit tests — `internal/domain/scoring/service_test.go`

- `TestNormalizeScore` — coefficient applied correctly for all 6 levels (A1–C2)
- `TestRecordScore_AccumulatesCorrectly` — two RecordScore calls → raw and normalized sum correctly

### Integration tests — `internal/telegram/handler_leaderboard_test.go`

- `TestHandleLeaderboard_OneGroup` — single group → leaderboard shown immediately
- `TestHandleLeaderboard_MultipleGroups` — multiple groups → inline group-selection keyboard shown
- `TestHandleLeaderboard_Callback` — `lb:` callback → leaderboard with scores displayed
- `TestHandleLeaderboard_MonthSwitch` — day↔month toggle works correctly

---

## Out of scope

- Weekly leaderboards
- Historical score recalculation when user level changes
- Leaderboard notifications (deferred to Phase 7 scheduler)
