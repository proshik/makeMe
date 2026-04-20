package task

import (
	"context"
	"time"
)

type Repository interface {
	CreateDailySet(ctx context.Context, userID, groupID int64, date time.Time, round int) (*DailySet, error)
	GetDailySet(ctx context.Context, userID, groupID int64, date time.Time) (*DailySet, error)
	AddTask(ctx context.Context, task *Task) error
	GetTasksByDailySet(ctx context.Context, dailySetID int64) ([]Task, error)
	GetTaskByID(ctx context.Context, taskID int64) (*Task, error)
	AddResult(ctx context.Context, result *TaskResult) error
	GetResult(ctx context.Context, taskID, userID int64) (*TaskResult, error)
	CountResults(ctx context.Context, dailySetID, userID int64) (int, int, error)        // answered, total
	CountCorrectResults(ctx context.Context, dailySetID, userID int64) (int, error) // correct
}
