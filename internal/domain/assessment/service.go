package assessment

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"makeMe/internal/domain/user"
)

const totalQuestions = 10

var (
	ErrNoActiveSession = errors.New("no active assessment session")
	ErrSessionComplete = errors.New("assessment session already completed")
)

// levels ordered by difficulty for navigation.
var levels = []user.Level{user.A1, user.A2, user.B1, user.B2, user.C1, user.C2}

type Service struct {
	repo    Repository
	userSvc *user.Service
}

func NewService(repo Repository, userSvc *user.Service) *Service {
	return &Service{repo: repo, userSvc: userSvc}
}

// Start creates a new assessment session starting at B1.
func (s *Service) Start(ctx context.Context, userID int64) (*Session, *Question, error) {
	// Check for existing active session
	existing, err := s.repo.GetActiveSession(ctx, userID)
	if err != nil && !errors.Is(err, ErrNoActiveSession) {
		return nil, nil, fmt.Errorf("check active session: %w", err)
	}
	if existing != nil {
		q := s.pickQuestion(existing.CurrentLevel)
		return existing, q, nil
	}

	session, err := s.repo.CreateSession(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("create session: %w", err)
	}

	q := s.pickQuestion(user.B1)
	return session, q, nil
}

// SubmitAnswer records the user's answer, adjusts difficulty, and returns the next question or nil if done.
func (s *Service) SubmitAnswer(ctx context.Context, userID int64, userOption int) (*Session, *Question, error) {
	session, err := s.repo.GetActiveSession(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get active session: %w", err)
	}
	if session.Status == StatusCompleted {
		return session, nil, ErrSessionComplete
	}

	// Get current question for this session level
	q := s.pickQuestion(session.CurrentLevel)

	correct := userOption == q.CorrectOption
	answer := &Answer{
		SessionID:     session.ID,
		QuestionLevel: session.CurrentLevel,
		QuestionText:  q.Text,
		Options:       q.Options,
		CorrectOption: q.CorrectOption,
		UserOption:    userOption,
		Correct:       correct,
	}
	if err := s.repo.AddAnswer(ctx, answer); err != nil {
		return nil, nil, fmt.Errorf("add answer: %w", err)
	}

	count, err := s.repo.CountAnswers(ctx, session.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("count answers: %w", err)
	}

	// Assessment complete?
	if count >= totalQuestions {
		return s.completeSession(ctx, session)
	}

	// Adjust level for next question
	nextLevel := adjustLevel(session.CurrentLevel, correct)
	session.CurrentLevel = nextLevel
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("update session level: %w", err)
	}

	nextQ := s.pickQuestion(nextLevel)
	return session, nextQ, nil
}

// GetActiveSession returns the current in-progress session or ErrNoActiveSession.
func (s *Service) GetActiveSession(ctx context.Context, userID int64) (*Session, error) {
	return s.repo.GetActiveSession(ctx, userID)
}

// GetCurrentQuestion returns the question for the session's current level.
func (s *Service) GetCurrentQuestion(ctx context.Context, session *Session) *Question {
	return s.pickQuestion(session.CurrentLevel)
}

// AnswerCount returns how many answers have been recorded for the session.
func (s *Service) AnswerCount(ctx context.Context, sessionID int64) (int, error) {
	return s.repo.CountAnswers(ctx, sessionID)
}

func (s *Service) completeSession(ctx context.Context, session *Session) (*Session, *Question, error) {
	stats, err := s.repo.CountCorrectByLevel(ctx, session.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("count correct by level: %w", err)
	}

	determined := determineLevel(stats)
	session.Status = StatusCompleted
	session.DeterminedLevel = determined

	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("complete session: %w", err)
	}

	if err := s.userSvc.UpdateLevel(ctx, session.UserID, determined); err != nil {
		return nil, nil, fmt.Errorf("update user level: %w", err)
	}

	return session, nil, nil
}

func (s *Service) pickQuestion(level user.Level) *Question {
	questions := questionBank[level]
	if len(questions) == 0 {
		questions = questionBank[user.B1]
	}
	q := questions[rand.IntN(len(questions))]
	return &q
}

// adjustLevel moves one step up on correct, one step down on incorrect.
func adjustLevel(current user.Level, correct bool) user.Level {
	idx := levelIndex(current)
	if correct && idx < len(levels)-1 {
		return levels[idx+1]
	}
	if !correct && idx > 0 {
		return levels[idx-1]
	}
	return current
}

func levelIndex(l user.Level) int {
	for i, lv := range levels {
		if lv == l {
			return i
		}
	}
	return 2 // default B1
}

// determineLevel picks the highest level where the user got >= 50% correct,
// falling back to A1 if none qualifies.
func determineLevel(stats map[string]LevelStats) user.Level {
	best := user.A1
	for i, lv := range levels {
		s, ok := stats[string(lv)]
		if !ok {
			continue
		}
		if s.Total > 0 && float64(s.Correct)/float64(s.Total) >= 0.5 {
			best = levels[i]
		}
	}
	return best
}
