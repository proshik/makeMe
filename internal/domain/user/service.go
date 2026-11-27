package user

import (
	"context"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("user not found")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Register creates a new user or updates an existing one (idempotent by telegram_id).
func (s *Service) Register(ctx context.Context, telegramID int64, username, firstName string) (*User, error) {
	u, err := s.repo.Create(ctx, telegramID, username, firstName)
	if err != nil {
		return nil, fmt.Errorf("register user: %w", err)
	}
	return u, nil
}

func (s *Service) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	u, err := s.repo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("get user by telegram id: %w", err)
	}
	return u, nil
}

// GetByID resolves a user by our own primary key. The web session carries this
// id in its JWT, unlike the bot which always starts from a telegram_id.
func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (s *Service) UpdateLevel(ctx context.Context, userID int64, level Level) error {
	if err := s.repo.UpdateLevel(ctx, userID, level); err != nil {
		return fmt.Errorf("update user level: %w", err)
	}
	return nil
}

// SetShowRuleOnCorrect stores whether the web interface shows the grammar rule
// after a correct answer as well as after a wrong one.
func (s *Service) SetShowRuleOnCorrect(ctx context.Context, userID int64, show bool) error {
	if err := s.repo.UpdateShowRuleOnCorrect(ctx, userID, show); err != nil {
		return fmt.Errorf("update show rule on correct: %w", err)
	}
	return nil
}
