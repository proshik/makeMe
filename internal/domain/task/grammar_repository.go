package task

import (
	"context"

	"makeMe/internal/domain/user"
)

// GrammarQuestion is a grammar exercise stored in the database.
type GrammarQuestion struct {
	ID          int64
	Level       user.Level
	Category    string
	Subcategory string
	Prompt      string
	Options     []string
	Answer      string
	Explanation string
}

// GrammarCategory is a grammar topic with its theory explanation.
type GrammarCategory struct {
	Name        string
	Explanation string
}

// GrammarRepository provides access to grammar questions.
type GrammarRepository interface {
	GetByLevel(ctx context.Context, level user.Level) ([]GrammarQuestion, error)
	GetCategoriesByLevel(ctx context.Context, level user.Level) ([]GrammarCategory, error)
	GetQuestionsByCategory(ctx context.Context, category string) ([]GrammarQuestion, error)
}
