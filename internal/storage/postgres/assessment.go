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
	return sessionFromFields(row.ID, row.UserID, row.Status, row.CurrentLevel,
		row.DeterminedLevel, row.PendingQuestion, row.CreatedAt, row.CompletedAt, row.ConsecutiveCorrect), nil
}

func (r *AssessmentRepository) GetActiveSession(ctx context.Context, userID int64) (*assessment.Session, error) {
	row, err := r.q.GetActiveAssessmentSession(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, assessment.ErrNoActiveSession
		}
		return nil, fmt.Errorf("get active session: %w", err)
	}
	return sessionFromFields(row.ID, row.UserID, row.Status, row.CurrentLevel,
		row.DeterminedLevel, row.PendingQuestion, row.CreatedAt, row.CompletedAt, row.ConsecutiveCorrect), nil
}

func (r *AssessmentRepository) UpdateSession(ctx context.Context, s *assessment.Session) error {
	params := sqlc.UpdateAssessmentSessionParams{
		ID:                 s.ID,
		Status:             string(s.Status),
		CurrentLevel:       string(s.CurrentLevel),
		ConsecutiveCorrect: int32(s.ConsecutiveCorrect),
	}

	if s.DeterminedLevel != "" {
		params.DeterminedLevel = pgtype.Text{String: string(s.DeterminedLevel), Valid: true}
	}

	if s.Status == assessment.StatusCompleted {
		params.CompletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	if s.PendingQuestion != nil {
		pqJSON, err := json.Marshal(s.PendingQuestion)
		if err != nil {
			return fmt.Errorf("marshal pending question: %w", err)
		}
		params.PendingQuestion = pqJSON
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

	userOption := int32(a.UserOption)
	if a.Skipped {
		userOption = int32(assessment.SkipOption)
	}

	params := sqlc.AddAssessmentAnswerParams{
		SessionID:     a.SessionID,
		QuestionLevel: string(a.QuestionLevel),
		QuestionText:  a.QuestionText,
		Options:       optionsJSON,
		CorrectOption: int32(a.CorrectOption),
		UserOption:    userOption,
		Correct:       a.Correct,
		Skipped:       a.Skipped,
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

func (r *AssessmentRepository) GetAnsweredTexts(ctx context.Context, sessionID int64) ([]string, error) {
	texts, err := r.q.GetAnsweredTexts(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get answered texts: %w", err)
	}
	return texts, nil
}

func (r *AssessmentRepository) GetSessionAnswers(ctx context.Context, sessionID int64) ([]*assessment.Answer, error) {
	rows, err := r.q.GetSessionAnswers(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session answers: %w", err)
	}

	answers := make([]*assessment.Answer, 0, len(rows))
	for _, row := range rows {
		var options []string
		if err := json.Unmarshal(row.Options, &options); err != nil {
			return nil, fmt.Errorf("unmarshal answer options: %w", err)
		}
		answers = append(answers, &assessment.Answer{
			ID:            row.ID,
			SessionID:     row.SessionID,
			QuestionLevel: user.Level(row.QuestionLevel),
			QuestionText:  row.QuestionText,
			Options:       options,
			CorrectOption: int(row.CorrectOption),
			UserOption:    int(row.UserOption),
			Correct:       row.Correct,
			Skipped:       row.Skipped,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return answers, nil
}

func (r *AssessmentRepository) GetLastCompleted(ctx context.Context, userID int64) (*assessment.Session, error) {
	row, err := r.q.GetLastCompletedAssessment(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, assessment.ErrNoActiveSession
		}
		return nil, fmt.Errorf("get last completed assessment: %w", err)
	}
	return sessionFromFields(row.ID, row.UserID, row.Status, row.CurrentLevel,
		row.DeterminedLevel, row.PendingQuestion, row.CreatedAt, row.CompletedAt, row.ConsecutiveCorrect), nil
}

// sessionFromFields constructs a domain Session from raw sqlc column values.
// Used by both CreateSession and GetActiveSession which return identically-shaped rows.
func sessionFromFields(
	id, userID int64,
	status, currentLevel string,
	determinedLevel pgtype.Text,
	pendingQuestionJSON []byte,
	createdAt, completedAt pgtype.Timestamptz,
	consecutiveCorrect int32,
) *assessment.Session {
	s := &assessment.Session{
		ID:                 id,
		UserID:             userID,
		Status:             assessment.SessionStatus(status),
		CurrentLevel:       user.Level(currentLevel),
		ConsecutiveCorrect: int(consecutiveCorrect),
		CreatedAt:          createdAt.Time,
	}
	if determinedLevel.Valid {
		s.DeterminedLevel = user.Level(determinedLevel.String)
	}
	if completedAt.Valid {
		s.CompletedAt = completedAt.Time
	}
	if len(pendingQuestionJSON) > 0 {
		var q assessment.Question
		if err := json.Unmarshal(pendingQuestionJSON, &q); err == nil {
			s.PendingQuestion = &q
		}
	}
	return s
}
