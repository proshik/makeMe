package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

const (
	vocabCount   = 3
	grammarCount = 2
	pointsPerTask = 10
)

var (
	ErrNoDailySet     = errors.New("no daily set for today")
	ErrAlreadyAnswered = errors.New("task already answered")
	ErrTaskNotFound    = errors.New("task not found")
)

type Service struct {
	repo       Repository
	srsRepo    srs.Repository
	generators []TaskGenerator
}

func NewService(repo Repository, srsRepo srs.Repository, generators ...TaskGenerator) *Service {
	return &Service{
		repo:       repo,
		srsRepo:    srsRepo,
		generators: generators,
	}
}

// GenerateDaily creates a daily set of tasks for a user in a group.
// Returns existing set if already generated for today.
func (s *Service) GenerateDaily(ctx context.Context, userID, groupID int64, level user.Level) (*DailySet, error) {
	today := time.Now().Truncate(24 * time.Hour)

	// Check if already exists
	existing, err := s.repo.GetDailySet(ctx, userID, groupID, today)
	if err == nil && existing != nil {
		tasks, err := s.repo.GetTasksByDailySet(ctx, existing.ID)
		if err != nil {
			return nil, fmt.Errorf("get tasks: %w", err)
		}
		existing.Tasks = tasks
		return existing, nil
	}

	ds, err := s.repo.CreateDailySet(ctx, userID, groupID, today)
	if err != nil {
		return nil, fmt.Errorf("create daily set: %w", err)
	}

	var allTasks []Task
	for _, gen := range s.generators {
		count := vocabCount
		if gen.Type() == TypeGrammar {
			count = grammarCount
		}

		tasks, err := gen.Generate(ctx, userID, level, count)
		if err != nil {
			return nil, fmt.Errorf("generate %s tasks: %w", gen.Type(), err)
		}
		allTasks = append(allTasks, tasks...)
	}

	for i := range allTasks {
		allTasks[i].DailySetID = ds.ID
		allTasks[i].Position = i
		if err := s.repo.AddTask(ctx, &allTasks[i]); err != nil {
			return nil, fmt.Errorf("add task: %w", err)
		}
	}

	ds.Tasks = allTasks
	return ds, nil
}

// GetTodaySet returns the daily set for today, or ErrNoDailySet.
func (s *Service) GetTodaySet(ctx context.Context, userID, groupID int64) (*DailySet, error) {
	today := time.Now().Truncate(24 * time.Hour)
	ds, err := s.repo.GetDailySet(ctx, userID, groupID, today)
	if err != nil {
		return nil, ErrNoDailySet
	}
	tasks, err := s.repo.GetTasksByDailySet(ctx, ds.ID)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	ds.Tasks = tasks
	return ds, nil
}

// GetTaskByID returns a single task by ID.
func (s *Service) GetTaskByID(ctx context.Context, taskID int64) (*Task, error) {
	return s.repo.GetTaskByID(ctx, taskID)
}

// SubmitAnswer records a user's answer and updates SRS if vocab.
func (s *Service) SubmitAnswer(ctx context.Context, taskID, userID int64, response string) (*TaskResult, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, ErrTaskNotFound
	}

	existing, err := s.repo.GetResult(ctx, taskID, userID)
	if err == nil && existing != nil {
		return nil, ErrAlreadyAnswered
	}

	correct := response == task.Answer
	score := 0
	if correct {
		score = pointsPerTask
	}

	result := &TaskResult{
		TaskID:   taskID,
		UserID:   userID,
		Response: response,
		Correct:  correct,
		Score:    score,
	}

	if err := s.repo.AddResult(ctx, result); err != nil {
		return nil, fmt.Errorf("add result: %w", err)
	}

	// Update SRS card for vocab tasks
	if task.Type == TypeVocab && task.Metadata != nil {
		if wordIDRaw, ok := task.Metadata["word_id"]; ok {
			wordID := toInt64(wordIDRaw)
			if wordID > 0 {
				s.updateSRS(ctx, userID, wordID, correct)
			}
		}
	}

	return result, nil
}

// GetNextUnanswered returns the next unanswered task in the daily set.
func (s *Service) GetNextUnanswered(ctx context.Context, dailySetID, userID int64) (*Task, error) {
	tasks, err := s.repo.GetTasksByDailySet(ctx, dailySetID)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}

	for _, t := range tasks {
		result, _ := s.repo.GetResult(ctx, t.ID, userID)
		if result == nil {
			return &t, nil
		}
	}

	return nil, nil // all answered
}

// GetProgress returns (answered, total) for a daily set.
func (s *Service) GetProgress(ctx context.Context, dailySetID, userID int64) (int, int, error) {
	return s.repo.CountResults(ctx, dailySetID, userID)
}

func (s *Service) updateSRS(ctx context.Context, userID, wordID int64, correct bool) {
	cards, _ := s.srsRepo.GetDueCards(ctx, userID, 100)
	var card *srs.ReviewCard
	for _, c := range cards {
		if c.WordID == wordID {
			card = &c
			break
		}
	}

	if card == nil {
		card = &srs.ReviewCard{
			UserID:     userID,
			WordID:     wordID,
			EaseFactor: 2.5,
		}
	}

	quality := srs.QualityFromCorrect(correct)
	updated := srs.Review(*card, quality, time.Now())
	s.srsRepo.UpsertCard(ctx, &updated)
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
