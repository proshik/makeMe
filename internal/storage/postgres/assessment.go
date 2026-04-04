package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/user"
	"makeMe/internal/storage/sqlc"
)

type AssessmentRepository struct {
	q *sqlc.Queries
}

func NewAssessmentRepository(q *sqlc.Queries) *AssessmentRepository {
	return &AssessmentRepository{q: q}
}

func (r *AssessmentRepository) CreateSession(ctx context.Context, userID int64) (*assessment.Session, error) {
	row, err := r.q.CreateAssessmentSession(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("create assessment session: %w", err)
	}
	return toSession(row), nil
}

func (r *AssessmentRepository) GetActiveSession(ctx context.Context, userID int64) (*assessment.Session, error) {
	row, err := r.q.GetActiveAssessmentSession(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, assessment.ErrNoActiveSession
		}
		return nil, fmt.Errorf("get active session: %w", err)
	}
	return toSession(row), nil
}

func (r *AssessmentRepository) UpdateSession(ctx context.Context, s *assessment.Session) error {
	params := sqlc.UpdateAssessmentSessionParams{
		ID:           s.ID,
		Status:       string(s.Status),
		CurrentLevel: string(s.CurrentLevel),
	}

	if s.DeterminedLevel != "" {
		params.DeterminedLevel = pgtype.Text{String: string(s.DeterminedLevel), Valid: true}
	}

	if s.Status == assessment.StatusCompleted {
		params.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	if err := r.q.UpdateAssessmentSession(ctx, params); err != nil {
		return fmt.Errorf("update assessment session: %w", err)
	}
	return nil
}

func (r *AssessmentRepository) AddAnswer(ctx context.Context, a *assessment.Answer) error {
	optionsJSON, err := json.Marshal(a.Options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}

	params := sqlc.AddAssessmentAnswerParams{
		SessionID:     a.SessionID,
		QuestionLevel: string(a.QuestionLevel),
		QuestionText:  a.QuestionText,
		Options:       optionsJSON,
		CorrectOption: int32(a.CorrectOption),
		UserOption:    int32(a.UserOption),
		Correct:       a.Correct,
	}

	if err := r.q.AddAssessmentAnswer(ctx, params); err != nil {
		return fmt.Errorf("add assessment answer: %w", err)
	}
	return nil
}

func (r *AssessmentRepository) CountAnswers(ctx context.Context, sessionID int64) (int, error) {
	count, err := r.q.CountAssessmentAnswers(ctx, sessionID)
	if err != nil {
		return 0, fmt.Errorf("count assessment answers: %w", err)
	}
	return int(count), nil
}

func (r *AssessmentRepository) CountCorrectByLevel(ctx context.Context, sessionID int64) (map[string]assessment.LevelStats, error) {
	rows, err := r.q.CountCorrectByLevel(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("count correct by level: %w", err)
	}

	result := make(map[string]assessment.LevelStats, len(rows))
	for _, row := range rows {
		result[row.QuestionLevel] = assessment.LevelStats{
			Correct: int(row.CorrectCount),
			Total:   int(row.Total),
		}
	}
	return result, nil
}

func toSession(row sqlc.AssessmentSession) *assessment.Session {
	s := &assessment.Session{
		ID:           row.ID,
		UserID:       row.UserID,
		Status:       assessment.SessionStatus(row.Status),
		CurrentLevel: user.Level(row.CurrentLevel),
		CreatedAt:    row.CreatedAt.Time,
	}
	if row.DeterminedLevel.Valid {
		s.DeterminedLevel = user.Level(row.DeterminedLevel.String)
	}
	if row.CompletedAt.Valid {
		s.CompletedAt = row.CompletedAt.Time
	}
	return s
}
