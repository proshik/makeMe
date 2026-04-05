package task

import (
	"context"

	"makeMe/internal/domain/user"
)

// TaskGenerator generates tasks of a specific type for a user.
type TaskGenerator interface {
	Type() TaskType
	Generate(ctx context.Context, userID int64, level user.Level, count int) ([]Task, error)
}
