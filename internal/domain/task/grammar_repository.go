package task

import (
	"context"

	"makeMe/internal/domain/user"
)

// GrammarQuestion is a grammar exercise stored in the database.
type GrammarQuestion struct {
	ID       int64
	Level    user.Level
	Category string
	Prompt   string
	Options  []string
	Answer   string
}

// GrammarRepository provides access to grammar questions.
type GrammarRepository interface {
	GetByLevel(ctx context.Context, level user.Level) ([]GrammarQuestion, error)
}
