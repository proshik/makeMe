package task

import (
	"context"
	"fmt"
	"math/rand/v2"

	"makeMe/internal/domain/user"
)

// GrammarGenerator generates grammar fill-in-the-blank tasks from the database.
type GrammarGenerator struct {
	repo GrammarRepository
}

func NewGrammarGenerator(repo GrammarRepository) *GrammarGenerator {
	return &GrammarGenerator{repo: repo}
}

func (g *GrammarGenerator) Type() TaskType { return TypeGrammar }

func (g *GrammarGenerator) Generate(ctx context.Context, _ int64, level user.Level, count int) ([]Task, error) {
	questions, err := g.repo.GetByLevel(ctx, level)
	if err != nil {
		return nil, fmt.Errorf("get grammar questions: %w", err)
	}

	// Fall back to B1 if level has no questions
	if len(questions) == 0 {
		questions, err = g.repo.GetByLevel(ctx, user.B1)
		if err != nil {
			return nil, fmt.Errorf("get grammar questions (fallback B1): %w", err)
		}
	}

	// Shuffle and pick
	perm := rand.Perm(len(questions))
	tasks := make([]Task, 0, count)
	for i := 0; i < count && i < len(perm); i++ {
		q := questions[perm[i]]
		tasks = append(tasks, Task{
			Type:    TypeGrammar,
			Level:   q.Level,
			Prompt:  q.Prompt,
			Options: q.Options,
			Answer:  q.Answer,
		})
	}

	return tasks, nil
}
