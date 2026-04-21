package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/storage/sqlc"
)

type GrammarRepository struct {
	q *sqlc.Queries
}

func NewGrammarRepository(q *sqlc.Queries) *GrammarRepository {
	return &GrammarRepository{q: q}
}

func (r *GrammarRepository) GetByLevel(ctx context.Context, level user.Level) ([]task.GrammarQuestion, error) {
	rows, err := r.q.GetGrammarByLevel(ctx, string(level))
	if err != nil {
		return nil, fmt.Errorf("get grammar by level: %w", err)
	}
	questions := make([]task.GrammarQuestion, 0, len(rows))
	for _, row := range rows {
		var options []string
		if err := json.Unmarshal(row.Options, &options); err != nil {
			return nil, fmt.Errorf("unmarshal grammar options: %w", err)
		}
		questions = append(questions, task.GrammarQuestion{
			ID:          row.ID,
			Level:       user.Level(row.Level),
			Category:    row.Category,
			Prompt:      row.Prompt,
			Options:     options,
			Answer:      row.Answer,
			Explanation: row.Explanation,
		})
	}
	return questions, nil
}

func (r *GrammarRepository) GetCategoriesByLevel(ctx context.Context, level user.Level) ([]task.GrammarCategory, error) {
	rows, err := r.q.GetGrammarCategoriesByLevel(ctx, string(level))
	if err != nil {
		return nil, fmt.Errorf("get grammar categories by level: %w", err)
	}
	cats := make([]task.GrammarCategory, len(rows))
	for i, row := range rows {
		cats[i] = task.GrammarCategory{Name: row.Category, Explanation: row.Explanation}
	}
	return cats, nil
}

func (r *GrammarRepository) GetQuestionsByCategory(ctx context.Context, category string, limit int) ([]task.GrammarQuestion, error) {
	rows, err := r.q.GetGrammarQuestionsByCategory(ctx, sqlc.GetGrammarQuestionsByCategoryParams{
		Category: category,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get grammar questions by category: %w", err)
	}
	questions := make([]task.GrammarQuestion, 0, len(rows))
	for _, row := range rows {
		var options []string
		if err := json.Unmarshal(row.Options, &options); err != nil {
			return nil, fmt.Errorf("unmarshal grammar options: %w", err)
		}
		questions = append(questions, task.GrammarQuestion{
			ID:       row.ID,
			Level:    user.Level(row.Level),
			Category: row.Category,
			Prompt:   row.Prompt,
			Options:  options,
			Answer:   row.Answer,
		})
	}
	return questions, nil
}
