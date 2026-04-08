package scoring

import "context"

type Repository interface {
	Upsert(ctx context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error
	GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]LeaderboardEntry, error)
	GetMonthlyScore(ctx context.Context, userID int64, periodKey string) (float64, error)
	GetScoreDays(ctx context.Context, userID int64) ([]string, error)
}
