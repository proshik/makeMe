# Scoring & Leaderboards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-task normalized scoring and group leaderboards (daily + monthly) accessible via `/leaderboard`.

**Architecture:** Separate `scoring` domain with its own service and repository; `handler_task.go` calls `scoringSvc.RecordScore` after each correct answer; new `handler_leaderboard.go` handles `/leaderboard` and `lb:` callbacks. `task.Task` gains a `GroupID` field populated via a JOIN in `GetTaskByID`.

**Tech Stack:** Go, PostgreSQL, sqlc, pgx/v5, go-telegram/bot, testcontainers (integration tests)

---

## File Map

| Action   | File                                              | Purpose                                         |
|----------|---------------------------------------------------|-------------------------------------------------|
| Create   | `migrations/00009_create_scoring.sql`             | `scores` table                                  |
| Create   | `queries/scoring.sql`                             | `UpsertScore`, `GetLeaderboard` queries         |
| Modify   | `queries/tasks.sql`                               | `GetTaskByID` — add JOIN to daily_sets          |
| Regen    | `internal/storage/sqlc/` (via `make sqlc`)        | Updated generated code                          |
| Modify   | `internal/domain/task/entity.go`                  | Add `GroupID int64` to `Task`                   |
| Modify   | `internal/storage/postgres/task.go`               | New `toTaskWithGroup` mapper for joined result  |
| Create   | `internal/domain/scoring/entity.go`               | `Score`, `LeaderboardEntry`                     |
| Create   | `internal/domain/scoring/repository.go`           | `Repository` interface                          |
| Create   | `internal/domain/scoring/service.go`              | `RecordScore`, `GetLeaderboard`, coefficients   |
| Create   | `internal/domain/scoring/service_test.go`         | Unit tests for normalization and upsert calls   |
| Create   | `internal/storage/postgres/scoring.go`            | `ScoringRepository` via sqlc                    |
| Modify   | `internal/telegram/bot.go`                        | Add `scoringSvc`, register `/leaderboard`/`lb:` |
| Modify   | `internal/telegram/testhelper_test.go`            | Wire `scoringSvc` into test env                 |
| Create   | `internal/telegram/handler_leaderboard.go`        | `/leaderboard` command and `lb:` callback       |
| Create   | `internal/telegram/handler_leaderboard_test.go`   | Integration tests for leaderboard               |
| Modify   | `internal/telegram/handler_task.go`               | Call `RecordScore` after correct answer         |
| Modify   | `cmd/makeme/main.go`                              | Wire `scoringRepo` and `scoringSvc`             |

---

## Task 1: Database migration + SQL queries + sqlc regeneration

**Files:**
- Create: `migrations/00009_create_scoring.sql`
- Create: `queries/scoring.sql`
- Modify: `queries/tasks.sql`
- Regen: `internal/storage/sqlc/` (run `make sqlc`)

- [ ] **Step 1: Create migration**

```sql
-- migrations/00009_create_scoring.sql
-- +goose Up
CREATE TABLE scores (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    group_id    BIGINT NOT NULL REFERENCES groups(id),
    period      TEXT NOT NULL CHECK (period IN ('day', 'month')),
    period_key  TEXT NOT NULL,
    raw         INT NOT NULL DEFAULT 0,
    normalized  FLOAT8 NOT NULL DEFAULT 0,
    UNIQUE (user_id, group_id, period, period_key)
);

-- +goose Down
DROP TABLE scores;
```

- [ ] **Step 2: Create scoring queries**

```sql
-- queries/scoring.sql

-- name: UpsertScore :exec
INSERT INTO scores (user_id, group_id, period, period_key, raw, normalized)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, group_id, period, period_key)
DO UPDATE SET
    raw        = scores.raw + EXCLUDED.raw,
    normalized = scores.normalized + EXCLUDED.normalized;

-- name: GetLeaderboard :many
SELECT u.id, u.first_name,
       COALESCE(s.raw, 0)::int       AS raw,
       COALESCE(s.normalized, 0.0)   AS normalized
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN scores s ON s.user_id = m.user_id
    AND s.group_id = m.group_id
    AND s.period     = $2
    AND s.period_key = $3
WHERE m.group_id = $1
ORDER BY normalized DESC;
```

- [ ] **Step 3: Update GetTaskByID query in queries/tasks.sql**

Replace the existing `GetTaskByID` query:

```sql
-- name: GetTaskByID :one
SELECT t.id, t.daily_set_id, t.type, t.level, t.prompt,
       t.options, t.answer, t.metadata, t.position,
       ds.group_id
FROM tasks t
JOIN daily_sets ds ON ds.id = t.daily_set_id
WHERE t.id = $1;
```

- [ ] **Step 4: Regenerate sqlc code**

```bash
make sqlc
```

Expected: no errors; `internal/storage/sqlc/` updated with new `Score` model, `UpsertScore`, `GetLeaderboard`, and `GetTaskByIDRow` types.

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: build errors in `internal/storage/postgres/task.go` because `GetTaskByID` now returns `GetTaskByIDRow` instead of `Task`. This is expected — will be fixed in Task 2.

- [ ] **Step 6: Commit**

```bash
git add migrations/00009_create_scoring.sql queries/scoring.sql queries/tasks.sql internal/storage/sqlc/
git commit -m "Add scoring migration, queries and regenerate sqlc"
```

---

## Task 2: Fix task.Task entity and storage mapper

**Files:**
- Modify: `internal/domain/task/entity.go`
- Modify: `internal/storage/postgres/task.go`

- [ ] **Step 1: Add GroupID to task.Task**

In `internal/domain/task/entity.go`, update the `Task` struct:

```go
type Task struct {
	ID         int64
	DailySetID int64
	GroupID    int64 // populated only when loaded via GetTaskByID
	Type       TaskType
	Level      user.Level
	Prompt     string
	Options    []string
	Answer     string
	Metadata   map[string]any
	Position   int
}
```

- [ ] **Step 2: Add toTaskWithGroup mapper and update GetTaskByID in postgres/task.go**

In `internal/storage/postgres/task.go`, replace the `GetTaskByID` method and add `toTaskWithGroup`:

```go
func (r *TaskRepository) GetTaskByID(ctx context.Context, taskID int64) (*task.Task, error) {
	row, err := r.q.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, task.ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	t := toTaskWithGroup(row)
	return &t, nil
}

func toTaskWithGroup(row sqlc.GetTaskByIDRow) task.Task {
	t := task.Task{
		ID:         row.ID,
		DailySetID: row.DailySetID,
		GroupID:    row.GroupID,
		Type:       task.TaskType(row.Type),
		Level:      user.Level(row.Level),
		Prompt:     row.Prompt,
		Answer:     row.Answer,
		Position:   int(row.Position),
	}
	if row.Options != nil {
		json.Unmarshal(row.Options, &t.Options)
	}
	if row.Metadata != nil {
		json.Unmarshal(row.Metadata, &t.Metadata)
	}
	return t
}
```

- [ ] **Step 3: Verify build and tests pass**

```bash
go build ./...
go test ./internal/domain/task/... ./internal/storage/... ./internal/telegram/...
```

Expected: build succeeds; all existing tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/task/entity.go internal/storage/postgres/task.go
git commit -m "Add GroupID to task.Task, update GetTaskByID mapper"
```

---

## Task 3: scoring domain — entity, repository interface, service + unit tests

**Files:**
- Create: `internal/domain/scoring/entity.go`
- Create: `internal/domain/scoring/repository.go`
- Create: `internal/domain/scoring/service_test.go`
- Create: `internal/domain/scoring/service.go`

- [ ] **Step 1: Create entity.go**

```go
// internal/domain/scoring/entity.go
package scoring

type Score struct {
	ID         int64
	UserID     int64
	GroupID    int64
	Period     string // "day" | "month"
	PeriodKey  string // "2026-04-06" | "2026-04"
	Raw        int
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

- [ ] **Step 2: Create repository.go**

```go
// internal/domain/scoring/repository.go
package scoring

import "context"

type Repository interface {
	Upsert(ctx context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error
	GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]LeaderboardEntry, error)
}
```

- [ ] **Step 3: Write failing unit tests**

```go
// internal/domain/scoring/service_test.go
package scoring

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

// fakeRepo records Upsert calls for inspection.
type fakeRepo struct {
	calls []upsertCall
}

type upsertCall struct {
	userID, groupID     int64
	period, periodKey   string
	rawDelta            int
	normDelta           float64
}

func (f *fakeRepo) Upsert(_ context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error {
	f.calls = append(f.calls, upsertCall{userID, groupID, period, periodKey, rawDelta, normDelta})
	return nil
}

func (f *fakeRepo) GetLeaderboard(_ context.Context, _ int64, _, _ string) ([]LeaderboardEntry, error) {
	return nil, nil
}

func TestNormalizeScore(t *testing.T) {
	cases := []struct {
		level    user.Level
		raw      int
		wantNorm float64
	}{
		{user.A1, 10, 6.0},
		{user.A2, 10, 8.0},
		{user.B1, 10, 10.0},
		{user.B2, 10, 11.5},
		{user.C1, 10, 13.0},
		{user.C2, 10, 15.0},
	}
	for _, tc := range cases {
		t.Run(string(tc.level), func(t *testing.T) {
			got := normalizeScore(tc.raw, tc.level)
			assert.Equal(t, tc.wantNorm, got)
		})
	}
}

func TestRecordScore_CallsUpsertForBothPeriods(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	err := svc.RecordScore(context.Background(), 1, 2, user.A1, 10)
	require.NoError(t, err)

	require.Len(t, repo.calls, 2, "should upsert day and month records")

	dayCall := repo.calls[0]
	assert.Equal(t, "day", dayCall.period)
	assert.Equal(t, time.Now().Format("2006-01-02"), dayCall.periodKey)
	assert.Equal(t, 10, dayCall.rawDelta)
	assert.Equal(t, 6.0, dayCall.normDelta) // A1 coefficient = 0.6

	monthCall := repo.calls[1]
	assert.Equal(t, "month", monthCall.period)
	assert.Equal(t, time.Now().Format("2006-01"), monthCall.periodKey)
	assert.Equal(t, 10, monthCall.rawDelta)
	assert.Equal(t, 6.0, monthCall.normDelta)
}

func TestRecordScore_UnknownLevel_UsesOneAsCoefficient(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	err := svc.RecordScore(context.Background(), 1, 1, user.Level("XX"), 10)
	require.NoError(t, err)

	require.NotEmpty(t, repo.calls)
	assert.Equal(t, 10.0, repo.calls[0].normDelta, "unknown level falls back to coefficient 1.0")
}

func TestNormalizeScore_ZeroRaw(t *testing.T) {
	got := normalizeScore(0, user.C2)
	assert.Equal(t, 0.0, got)
}

// Compile-time check that fakeRepo implements Repository.
var _ Repository = (*fakeRepo)(nil)
```

- [ ] **Step 4: Run tests — verify they fail**

```bash
go test ./internal/domain/scoring/...
```

Expected: compilation error — `normalizeScore`, `NewService`, `Service` not defined yet.

- [ ] **Step 5: Implement service.go**

```go
// internal/domain/scoring/service.go
package scoring

import (
	"context"
	"time"

	"makeMe/internal/domain/user"
)

var coefficients = map[user.Level]float64{
	user.A1: 0.6,
	user.A2: 0.8,
	user.B1: 1.0,
	user.B2: 1.15,
	user.C1: 1.3,
	user.C2: 1.5,
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// RecordScore adds rawPoints to both the daily and monthly score records for
// the given user+group, normalizing by the task level coefficient.
func (s *Service) RecordScore(ctx context.Context, userID, groupID int64, taskLevel user.Level, rawPoints int) error {
	norm := normalizeScore(rawPoints, taskLevel)
	now := time.Now()

	if err := s.repo.Upsert(ctx, userID, groupID, "day", now.Format("2006-01-02"), rawPoints, norm); err != nil {
		return err
	}
	return s.repo.Upsert(ctx, userID, groupID, "month", now.Format("2006-01"), rawPoints, norm)
}

// GetLeaderboard returns all group members ranked by normalized score for
// the given period and period key.
func (s *Service) GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]LeaderboardEntry, error) {
	return s.repo.GetLeaderboard(ctx, groupID, period, periodKey)
}

func normalizeScore(raw int, level user.Level) float64 {
	coeff, ok := coefficients[level]
	if !ok {
		coeff = 1.0
	}
	return float64(raw) * coeff
}
```

- [ ] **Step 6: Run tests — verify they pass**

```bash
go test ./internal/domain/scoring/... -v
```

Expected:
```
--- PASS: TestNormalizeScore/A1
--- PASS: TestNormalizeScore/A2
--- PASS: TestNormalizeScore/B1
--- PASS: TestNormalizeScore/B2
--- PASS: TestNormalizeScore/C1
--- PASS: TestNormalizeScore/C2
--- PASS: TestRecordScore_CallsUpsertForBothPeriods
--- PASS: TestRecordScore_UnknownLevel_UsesOneAsCoefficient
--- PASS: TestNormalizeScore_ZeroRaw
PASS
```

- [ ] **Step 7: Commit**

```bash
git add internal/domain/scoring/
git commit -m "Add scoring domain: entity, repository interface, service with unit tests"
```

---

## Task 4: scoring storage implementation

**Files:**
- Create: `internal/storage/postgres/scoring.go`

- [ ] **Step 1: Implement ScoringRepository**

```go
// internal/storage/postgres/scoring.go
package postgres

import (
	"context"
	"fmt"

	"makeMe/internal/domain/scoring"
	"makeMe/internal/storage/sqlc"
)

type ScoringRepository struct {
	q *sqlc.Queries
}

func NewScoringRepository(q *sqlc.Queries) *ScoringRepository {
	return &ScoringRepository{q: q}
}

func (r *ScoringRepository) Upsert(ctx context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error {
	err := r.q.UpsertScore(ctx, sqlc.UpsertScoreParams{
		UserID:     userID,
		GroupID:    groupID,
		Period:     period,
		PeriodKey:  periodKey,
		Raw:        int32(rawDelta),
		Normalized: normDelta,
	})
	if err != nil {
		return fmt.Errorf("upsert score: %w", err)
	}
	return nil
}

func (r *ScoringRepository) GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]scoring.LeaderboardEntry, error) {
	rows, err := r.q.GetLeaderboard(ctx, sqlc.GetLeaderboardParams{
		GroupID:   groupID,
		Period:    period,
		PeriodKey: periodKey,
	})
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}

	entries := make([]scoring.LeaderboardEntry, len(rows))
	for i, row := range rows {
		entries[i] = scoring.LeaderboardEntry{
			UserID:     row.ID,
			FirstName:  row.FirstName,
			Raw:        int(row.Raw),
			Normalized: row.Normalized,
			Rank:       i + 1,
		}
	}
	return entries, nil
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/scoring.go
git commit -m "Add postgres ScoringRepository"
```

---

## Task 5: Wire scoring into bot and update test helper

**Files:**
- Modify: `internal/telegram/bot.go`
- Modify: `internal/telegram/testhelper_test.go`
- Modify: `cmd/makeme/main.go`

- [ ] **Step 1: Update bot.go — add scoringSvc to Bot struct and New()**

In `internal/telegram/bot.go`, update imports and the struct:

```go
import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/dictionary"
	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
)

type Bot struct {
	bot        *bot.Bot
	userSvc    *user.Service
	groupSvc   *group.Service
	assessSvc  *assessment.Service
	taskSvc    *task.Service
	scoringSvc *scoring.Service
	srsRepo    srs.Repository
	dictClient *dictionary.Client
}
```

Update `New()` signature and body:

```go
func New(token string, userSvc *user.Service, groupSvc *group.Service, assessSvc *assessment.Service, taskSvc *task.Service, scoringSvc *scoring.Service, srsRepo srs.Repository, dictClient *dictionary.Client, extraOpts ...bot.Option) (*Bot, error) {
	b := &Bot{
		userSvc:    userSvc,
		groupSvc:   groupSvc,
		assessSvc:  assessSvc,
		taskSvc:    taskSvc,
		scoringSvc: scoringSvc,
		srsRepo:    srsRepo,
		dictClient: dictClient,
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.defaultHandler),
		bot.WithMiddlewares(b.userResolveMiddleware),
	}
	opts = append(opts, extraOpts...)

	tgBot, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/assess", bot.MatchTypeExact, b.handleAssess)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/newgroup ", bot.MatchTypePrefix, b.handleNewGroup)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/join ", bot.MatchTypePrefix, b.handleJoin)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/mygroups", bot.MatchTypeExact, b.handleMyGroups)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/tasks", bot.MatchTypeExact, b.handleTasks)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/leaderboard", bot.MatchTypeExact, b.handleLeaderboard)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "assess:", bot.MatchTypePrefix, b.handleAssessCallback)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "task:", bot.MatchTypePrefix, b.handleTaskCallback)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "word_audio:", bot.MatchTypePrefix, b.handleWordAudioCallback)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "lb:", bot.MatchTypePrefix, b.handleLeaderboardCallback)

	b.bot = tgBot
	return b, nil
}
```

- [ ] **Step 2: Update cmd/makeme/main.go**

Add scoring imports and wiring:

```go
import (
	// existing imports ...
	"makeMe/internal/domain/scoring"
)

// inside run(), after taskSvc:
scoringRepo := postgres.NewScoringRepository(queries)
scoringSvc := scoring.NewService(scoringRepo)

// update telegram.New call:
tgBot, err := telegram.New(cfg.TelegramToken, userSvc, groupSvc, assessSvc, taskSvc, scoringSvc, srsRepo, dictClient)
```

- [ ] **Step 3: Update testhelper_test.go**

Add scoring fields to `testEnv` struct:

```go
type testEnv struct {
	pool        *pgxpool.Pool
	queries     *sqlc.Queries
	userSvc     *user.Service
	groupSvc    *group.Service
	assessSvc   *assessment.Service
	taskSvc     *task.Service
	scoringSvc  *scoring.Service
	srsRepo     *postgres.SRSRepository
	tgBot       *telegram.Bot
	webhookURL  string
	wiremockURL string
	httpServer  *httptest.Server
}
```

Add scoring import and wiring in `setupTestEnv`:

```go
import (
	// existing imports ...
	"makeMe/internal/domain/scoring"
)

// inside setupTestEnv(), after taskSvc:
scoringRepo := postgres.NewScoringRepository(queries)
scoringSvc := scoring.NewService(scoringRepo)

// update telegram.New call:
tgBot, err := telegram.New("test-token", userSvc, groupSvc, assessSvc, taskSvc, scoringSvc, srsRepo, dictClient,
	bot.WithServerURL(wiremockURL),
)

// update returned struct:
return &testEnv{
	// ... existing fields ...
	scoringSvc:  scoringSvc,
}
```

- [ ] **Step 4: Verify build and all existing tests pass**

```bash
go build ./...
go test ./internal/telegram/... -timeout 300s
```

Expected: all existing tests pass (scoring is wired but not yet called from handlers).

- [ ] **Step 5: Commit**

```bash
git add internal/telegram/bot.go internal/telegram/testhelper_test.go cmd/makeme/main.go
git commit -m "Wire scoring service into bot, test helper and main"
```

---

## Task 6: Leaderboard handler with integration tests (TDD)

**Files:**
- Create: `internal/telegram/handler_leaderboard_test.go`
- Create: `internal/telegram/handler_leaderboard.go`

- [ ] **Step 1: Write failing integration tests**

```go
// internal/telegram/handler_leaderboard_test.go
package telegram_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

func TestHandleLeaderboard_OneGroup(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 921, "lbuser1", "LBUser1")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 921, 921, "lbuser1", "LBUser1", "/newgroup MyGroup"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 921, 921, "lbuser1", "LBUser1", "/leaderboard"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	last := reqs[len(reqs)-1].Body
	assert.Contains(t, last, "MyGroup")
	assert.Contains(t, last, "LBUser1")
}

func TestHandleLeaderboard_MultipleGroups(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 922, "lbuser2", "LBUser2")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 922, 922, "lbuser2", "LBUser2", "/newgroup Alpha"))
	env.sendUpdate(t, makeTextUpdate(2, 922, 922, "lbuser2", "LBUser2", "/newgroup Beta"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(3, 922, 922, "lbuser2", "LBUser2", "/leaderboard"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	last := reqs[len(reqs)-1].Body
	// Bot should ask to choose a group
	assert.Contains(t, last, "Alpha")
	assert.Contains(t, last, "Beta")
	assert.Contains(t, last, "inline_keyboard")
}

func TestHandleLeaderboard_Callback(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 923, "lbuser3", "LBUser3")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 923, 923, "lbuser3", "LBUser3", "/newgroup ScoreGroup"))

	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, groups)
	groupID := groups[0].ID

	// Seed a score directly
	err = env.scoringSvc.RecordScore(context.Background(), u.ID, groupID, user.B1, 10)
	require.NoError(t, err)

	env.resetWireMock(t)

	periodKey := time.Now().Format("2006-01-02")
	env.sendUpdate(t, makeCallbackUpdate(923, 923, 923, "lbuser3", "LBUser3",
		fmt.Sprintf("lb:%d:day:%s", groupID, periodKey)))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	last := reqs[len(reqs)-1].Body
	assert.Contains(t, last, "LBUser3")
	assert.Contains(t, last, "очков")
}

func TestHandleLeaderboard_MonthSwitch(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 924, "lbuser4", "LBUser4")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "A2")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 924, 924, "lbuser4", "LBUser4", "/newgroup ToggleGroup"))

	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	env.resetWireMock(t)

	// Day view — should contain "За месяц" toggle button
	periodKey := time.Now().Format("2006-01-02")
	env.sendUpdate(t, makeCallbackUpdate(924, 924, 924, "lbuser4", "LBUser4",
		fmt.Sprintf("lb:%d:day:%s", groupID, periodKey)))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	dayResp := reqs[len(reqs)-1].Body
	assert.Contains(t, dayResp, "месяц")

	env.resetWireMock(t)

	// Month view — should contain "За сегодня" toggle button
	monthKey := time.Now().Format("2006-01")
	env.sendUpdate(t, makeCallbackUpdate(925, 924, 924, "lbuser4", "LBUser4",
		fmt.Sprintf("lb:%d:month:%s", groupID, monthKey)))

	reqs = env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	monthResp := reqs[len(reqs)-1].Body
	assert.Contains(t, monthResp, "сегодня")
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/telegram/... -run "TestHandleLeaderboard" -timeout 300s
```

Expected: compilation error — `handleLeaderboard`, `handleLeaderboardCallback` not defined.

- [ ] **Step 3: Implement handler_leaderboard.go**

```go
// internal/telegram/handler_leaderboard.go
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/group"
)

func (b *Bot) handleLeaderboard(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil || len(groups) == 0 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "У тебя нет групп. Создай группу (/newgroup) или вступи в существующую (/join).",
		})
		return
	}

	if len(groups) == 1 {
		periodKey := time.Now().Format("2006-01-02")
		b.sendLeaderboard(ctx, tgBot, update.Message.Chat.ID, groups[0], "day", periodKey)
		return
	}

	// Multiple groups — ask which one
	periodKey := time.Now().Format("2006-01-02")
	buttons := make([][]models.InlineKeyboardButton, len(groups))
	for i, g := range groups {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: g.Name, CallbackData: fmt.Sprintf("lb:%d:day:%s", g.ID, periodKey)},
		}
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Выбери группу:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func (b *Bot) handleLeaderboardCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	// Parse "lb:{groupID}:{period}:{periodKey}"
	parts := strings.SplitN(update.CallbackQuery.Data, ":", 4)
	if len(parts) != 4 {
		return
	}

	groupID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}
	period := parts[2]
	periodKey := parts[3]

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil {
		slog.Error("failed to get groups for leaderboard", "user_id", u.ID, "error", err)
		return
	}

	var selected *group.Group
	for _, g := range groups {
		if g.ID == groupID {
			selected = &g
			break
		}
	}
	if selected == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	b.sendLeaderboard(ctx, tgBot, chatID, *selected, period, periodKey)
}

func (b *Bot) sendLeaderboard(ctx context.Context, tgBot *bot.Bot, chatID int64, g group.Group, period, periodKey string) {
	entries, err := b.scoringSvc.GetLeaderboard(ctx, g.ID, period, periodKey)
	if err != nil {
		slog.Error("failed to get leaderboard", "group_id", g.ID, "error", err)
		return
	}

	var periodLabel string
	if period == "day" {
		periodLabel = "сегодня"
	} else {
		periodLabel = fmt.Sprintf("месяц %s", periodKey)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🏆 Группа «%s» — %s\n\n", g.Name, periodLabel))
	if len(entries) == 0 {
		sb.WriteString("Пока нет результатов.")
	} else {
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("%d. %s — %.1f очков\n", e.Rank, e.FirstName, e.Normalized))
		}
	}

	var toggleText, togglePeriod, toggleKey string
	if period == "day" {
		toggleText = "📅 За месяц"
		togglePeriod = "month"
		toggleKey = time.Now().Format("2006-01")
	} else {
		toggleText = "📅 За сегодня"
		togglePeriod = "day"
		toggleKey = time.Now().Format("2006-01-02")
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: toggleText, CallbackData: fmt.Sprintf("lb:%d:%s:%s", g.ID, togglePeriod, toggleKey)}},
			},
		},
	})
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/telegram/... -run "TestHandleLeaderboard" -timeout 300s -v
```

Expected:
```
--- PASS: TestHandleLeaderboard_OneGroup
--- PASS: TestHandleLeaderboard_MultipleGroups
--- PASS: TestHandleLeaderboard_Callback
--- PASS: TestHandleLeaderboard_MonthSwitch
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/telegram/handler_leaderboard.go internal/telegram/handler_leaderboard_test.go
git commit -m "Add leaderboard handler with integration tests"
```

---

## Task 7: Call RecordScore from task handler

**Files:**
- Modify: `internal/telegram/handler_task.go`

- [ ] **Step 1: Add RecordScore call in handleTaskCallback**

In `internal/telegram/handler_task.go`, after sending feedback to the user, add scoring. Find the section after `result, err := b.taskSvc.SubmitAnswer(...)` and update:

```go
	// Feedback
	var feedback string
	if result.Correct {
		feedback = "✅ Правильно!"
		if b.scoringSvc != nil {
			if err := b.scoringSvc.RecordScore(ctx, u.ID, t.GroupID, t.Level, result.Score); err != nil {
				slog.Error("failed to record score", "user_id", u.ID, "task_id", taskID, "error", err)
			}
		}
	} else {
		feedback = fmt.Sprintf("❌ Неправильно. Правильный ответ: %s", t.Answer)
	}
```

- [ ] **Step 2: Run all tests**

```bash
go test ./... -timeout 300s
```

Expected: all tests pass, including the pre-existing `TestHandleTasks_FullFlow`.

- [ ] **Step 3: Commit**

```bash
git add internal/telegram/handler_task.go
git commit -m "Call RecordScore after each correct task answer"
```

---

## Task 8: Update PLAN.md

**Files:**
- Modify: `PLAN.md`

- [ ] **Step 1: Mark Phase 6 as DONE in PLAN.md**

In `PLAN.md`, update the Phase 6 header and mark all checklist items:

```markdown
### Фаза 6 — Баллы и таблицы лидеров [DONE]

- [x] `migrations/00009_create_scoring.sql` — таблица `scores` (FLOAT8 для normalized)
- [x] `internal/domain/scoring/entity.go` — Score, LeaderboardEntry
- [x] `internal/domain/scoring/repository.go` — интерфейс: `Upsert`, `GetLeaderboard`
- [x] `internal/domain/scoring/service.go` — `RecordScore` (нормализация + upsert day/month), `GetLeaderboard`
- [x] `internal/domain/scoring/service_test.go` — unit-тесты: нормализация всех уровней, два вызова Upsert per RecordScore
- [x] `internal/storage/postgres/scoring.go` — реализация через sqlc
- [x] `queries/scoring.sql` — `UpsertScore`, `GetLeaderboard` (LEFT JOIN от memberships)
- [x] `internal/domain/task/entity.go` — добавлен `GroupID` в `Task`
- [x] `internal/storage/postgres/task.go` — `GetTaskByID` использует JOIN с `daily_sets`
- [x] `internal/telegram/handler_leaderboard.go` — `/leaderboard`, callback `lb:`, переключение day/month
- [x] `internal/telegram/handler_leaderboard_test.go` — integration-тесты: одна группа, несколько групп, callback, переключение
- [x] `internal/telegram/handler_task.go` — вызов `scoringSvc.RecordScore` при правильном ответе
- [x] `internal/telegram/bot.go` — добавлен `scoringSvc`, зарегистрированы `/leaderboard` и `lb:`
- [x] `cmd/makeme/main.go` — подключён scoringRepo и scoringSvc
```

- [ ] **Step 2: Commit**

```bash
git add PLAN.md
git commit -m "Mark Phase 6 as complete in PLAN.md"
```
