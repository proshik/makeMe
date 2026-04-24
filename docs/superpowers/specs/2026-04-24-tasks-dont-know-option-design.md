# /tasks "🤷 Не знаю" option

## Problem

`/assess` already offers users a **🤷 Не знаю** button so they can honestly admit they
don't know an answer instead of guessing randomly. `/tasks` has no such option — the
user must pick one of the four answer buttons, which encourages random guessing,
pollutes SRS signals for vocabulary, and feels inconsistent with the onboarding test.

## Goal

Add the same **🤷 Не знаю** affordance to every `/tasks` question (vocab + grammar)
so users can skip with an honest signal that is recorded separately from a wrong
guess.

## User-visible behaviour

- Each task card gets one extra keyboard row: **🤷 Не знаю** (last row).
- Tapping it:
  - Records a skipped result (0 points, `correct=false`, `skipped=true`).
  - Shows feedback: `🤷 Пропущено. Правильный ответ: <b>X</b>` — plus the grammar
    explanation (if `Metadata["explanation"]` is set), just like the wrong-answer flow.
  - Advances to the next task.
- Progress counter and round logic behave exactly as they do for a normal answer.

## Design

### Schema

Migration `00021_add_task_results_skipped.sql`:

```sql
-- +goose Up
ALTER TABLE task_results ADD COLUMN skipped BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE task_results DROP COLUMN skipped;
```

### Domain

`internal/domain/task/entity.go` — add `Skipped bool` to `TaskResult`.

`internal/domain/task/service.go` — new method:

```go
func (s *Service) SubmitSkip(ctx, taskID, userID int64) (*TaskResult, error)
```

Behaviour:

- Loads the task; returns `ErrTaskNotFound` if missing.
- Returns `ErrAlreadyAnswered` if the user already has a result for this task.
- Persists `TaskResult{Response: "", Correct: false, Score: 0, Skipped: true}`.
- For `TypeVocab` tasks with a `word_id` in metadata, calls `updateSRS(..., false)`
  so SM-2 treats the skip the same as a wrong answer for review scheduling.

A new method (not a sentinel in `SubmitAnswer`) keeps the public API unambiguous
and avoids the risk of a real option string ever colliding with the sentinel.

### SQL / sqlc

`queries/tasks.sql`:

- Update `AddResult` to `INSERT INTO task_results (task_id, user_id, response, correct, score, skipped)`
  with a sixth parameter.
- Update `GetResult` `SELECT` (usually `SELECT *` — confirm and adjust if explicit
  column list).
- `make sqlc` to regenerate.

`internal/storage/taskrepo.go` (or wherever `AddResult` is wired): pass
`result.Skipped` through to the sqlc call.

### Telegram layer

`internal/telegram/handler_task.go`:

- `sendTaskQuestion` appends a trailing row
  `{Text: "🤷 Не знаю", CallbackData: "task:<taskID>:idk"}`.
- `handleTaskCallback` branches on `parts[2]`:
  - `"idk"` → call `b.taskSvc.SubmitSkip(ctx, taskID, u.ID)`, build skip feedback.
  - otherwise → existing `SubmitAnswer` path.
- Skip feedback helper `buildSkipFeedback(t *task.Task) string` mirrors
  `buildIncorrectFeedback` but leads with `🤷 Пропущено.` instead of
  `❌ Неправильно.`.

## Testing

### Unit — `internal/domain/task/service_test.go`

- `TestSubmitSkip_RecordsSkippedResult` — stored result has
  `Skipped=true, Correct=false, Score=0, Response=""`.
- `TestSubmitSkip_UpdatesSRS_AsWrong_ForVocab` — fake SRS repo captures one
  `UpsertCard` call with SM-2 quality matching a failed review.
- `TestSubmitSkip_ReturnsErrAlreadyAnswered` — second skip on the same task fails.
- `TestSubmitSkip_ReturnsErrTaskNotFound` — unknown task id.

### Integration — `internal/telegram/handler_task_test.go`

- `TestHandleTasks_ShowsIdkButton` — first `/tasks` message's keyboard includes a
  button with text `🤷 Не знаю` and callback suffix `:idk`.
- `TestHandleTaskCallback_Skip_RecordsAndShowsAnswer` — sending a `task:<id>:idk`
  callback results in a message containing `Пропущено` and the task's correct answer;
  DB has a `task_results` row with `skipped=true, correct=false, score=0`.

## Out of scope

- Scoring changes (skip stays 0 points, identical to wrong).
- `/profile` stats split for skipped vs wrong.
- Leaderboard impact (unchanged — only `correct=true` scores were ever awarded).
- UI for re-attempting a skipped task later.
