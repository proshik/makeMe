package scoring

import (
	"context"
	"time"

	"makeMe/internal/domain/user"
)

var coefficients = map[user.Level]float64{
	user.A1: 0.6,
	user.A2: 0.8,
	user.B1: 1.0,
	user.B2: 1.15,
	user.C1: 1.3,
	user.C2: 1.5,
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordScore(ctx context.Context, userID, groupID int64, taskLevel user.Level, rawPoints int) error {
	norm := normalizeScore(rawPoints, taskLevel)
	now := time.Now()
	if err := s.repo.Upsert(ctx, userID, groupID, "day", now.Format("2006-01-02"), rawPoints, norm); err != nil {
		return err
	}
	return s.repo.Upsert(ctx, userID, groupID, "month", now.Format("2006-01"), rawPoints, norm)
}

func (s *Service) GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]LeaderboardEntry, error) {
	return s.repo.GetLeaderboard(ctx, groupID, period, periodKey)
}

func normalizeScore(raw int, level user.Level) float64 {
	coeff, ok := coefficients[level]
	if !ok {
		coeff = 1.0
	}
	return float64(raw) * coeff
}
