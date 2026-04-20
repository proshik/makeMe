package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/storage/sqlc"
)

type TaskRepository struct {
	q *sqlc.Queries
}

func NewTaskRepository(q *sqlc.Queries) *TaskRepository {
	return &TaskRepository{q: q}
}

func (r *TaskRepository) CreateDailySet(ctx context.Context, userID, groupID int64, date time.Time, round int) (*task.DailySet, error) {
	row, err := r.q.CreateDailySet(ctx, sqlc.CreateDailySetParams{
		UserID:  userID,
		GroupID: groupID,
		Date:    pgtype.Date{Time: date, Valid: true},
		Round:   int32(round),
	})
	if err != nil {
		return nil, fmt.Errorf("create daily set: %w", err)
	}
	return toDailySet(row), nil
}

func (r *TaskRepository) GetDailySet(ctx context.Context, userID, groupID int64, date time.Time) (*task.DailySet, error) {
	row, err := r.q.GetLatestDailySet(ctx, sqlc.GetLatestDailySetParams{
		UserID:  userID,
		GroupID: groupID,
		Date:    pgtype.Date{Time: date, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, task.ErrNoDailySet
		}
		return nil, fmt.Errorf("get daily set: %w", err)
	}
	return toDailySet(row), nil
}

func (r *TaskRepository) AddTask(ctx context.Context, t *task.Task) error {
	optionsJSON, _ := json.Marshal(t.Options)
	metadataJSON, _ := json.Marshal(t.Metadata)

	row, err := r.q.AddTask(ctx, sqlc.AddTaskParams{
		DailySetID: t.DailySetID,
		Type:       string(t.Type),
		Level:      string(t.Level),
		Prompt:     t.Prompt,
		Options:    optionsJSON,
		Answer:     t.Answer,
		Metadata:   metadataJSON,
		Position:   int32(t.Position),
	})
	if err != nil {
		return fmt.Errorf("add task: %w", err)
	}
	t.ID = row.ID
	return nil
}

func (r *TaskRepository) GetTasksByDailySet(ctx context.Context, dailySetID int64) ([]task.Task, error) {
	rows, err := r.q.GetTasksByDailySet(ctx, dailySetID)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}

	tasks := make([]task.Task, len(rows))
	for i, row := range rows {
		tasks[i] = toTask(row)
	}
	return tasks, nil
}

func (r *TaskRepository) GetTaskByID(ctx context.Context, taskID int64) (*task.Task, error) {
	row, err := r.q.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, task.ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	t := toTask(sqlc.Task{
		ID:         row.ID,
		DailySetID: row.DailySetID,
		Type:       row.Type,
		Level:      row.Level,
		Prompt:     row.Prompt,
		Options:    row.Options,
		Answer:     row.Answer,
		Metadata:   row.Metadata,
		Position:   row.Position,
	})
	t.GroupID = row.GroupID
	return &t, nil
}

func (r *TaskRepository) AddResult(ctx context.Context, result *task.TaskResult) error {
	row, err := r.q.AddTaskResult(ctx, sqlc.AddTaskResultParams{
		TaskID:   result.TaskID,
		UserID:   result.UserID,
		Response: result.Response,
		Correct:  result.Correct,
		Score:    int32(result.Score),
	})
	if err != nil {
		return fmt.Errorf("add task result: %w", err)
	}
	result.ID = row.ID
	return nil
}

func (r *TaskRepository) GetResult(ctx context.Context, taskID, userID int64) (*task.TaskResult, error) {
	row, err := r.q.GetTaskResult(ctx, sqlc.GetTaskResultParams{
		TaskID: taskID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get task result: %w", err)
	}
	return toTaskResult(row), nil
}

func (r *TaskRepository) CountResults(ctx context.Context, dailySetID, userID int64) (int, int, error) {
	row, err := r.q.CountTaskResults(ctx, sqlc.CountTaskResultsParams{
		DailySetID: dailySetID,
		UserID:     userID,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("count results: %w", err)
	}
	return int(row.Answered), int(row.Total), nil
}

func (r *TaskRepository) CountCorrectResults(ctx context.Context, dailySetID, userID int64) (int, error) {
	count, err := r.q.CountCorrectTaskResults(ctx, sqlc.CountCorrectTaskResultsParams{
		DailySetID: dailySetID,
		UserID:     userID,
	})
	if err != nil {
		return 0, fmt.Errorf("count correct results: %w", err)
	}
	return int(count), nil
}

func toDailySet(row sqlc.DailySet) *task.DailySet {
	return &task.DailySet{
		ID:        row.ID,
		UserID:    row.UserID,
		GroupID:   row.GroupID,
		Date:      row.Date.Time,
		CreatedAt: row.CreatedAt.Time,
		Round:     int(row.Round),
	}
}

func toTask(row sqlc.Task) task.Task {
	t := task.Task{
		ID:         row.ID,
		DailySetID: row.DailySetID,
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

func toTaskResult(row sqlc.TaskResult) *task.TaskResult {
	return &task.TaskResult{
		ID:       row.ID,
		TaskID:   row.TaskID,
		UserID:   row.UserID,
		Response: row.Response,
		Correct:  row.Correct,
		Score:    int(row.Score),
	}
}
