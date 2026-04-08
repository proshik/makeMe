package scoring

import "context"

type Repository interface {
	Upsert(ctx context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error
	GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]LeaderboardEntry, error)
}
