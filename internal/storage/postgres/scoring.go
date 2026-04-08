package postgres

import (
	"context"
	"fmt"

	"makeMe/internal/domain/scoring"
	"makeMe/internal/storage/sqlc"
)

type ScoringRepository struct {
	q *sqlc.Queries
}

func NewScoringRepository(q *sqlc.Queries) *ScoringRepository {
	return &ScoringRepository{q: q}
}

func (r *ScoringRepository) Upsert(ctx context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error {
	err := r.q.UpsertScore(ctx, sqlc.UpsertScoreParams{
		UserID:     userID,
		GroupID:    groupID,
		Period:     period,
		PeriodKey:  periodKey,
		Raw:        int32(rawDelta),
		Normalized: normDelta,
	})
	if err != nil {
		return fmt.Errorf("upsert score: %w", err)
	}
	return nil
}

func (r *ScoringRepository) GetLeaderboard(ctx context.Context, groupID int64, period, periodKey string) ([]scoring.LeaderboardEntry, error) {
	rows, err := r.q.GetLeaderboard(ctx, sqlc.GetLeaderboardParams{
		GroupID:   groupID,
		Period:    period,
		PeriodKey: periodKey,
	})
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}

	entries := make([]scoring.LeaderboardEntry, len(rows))
	for i, row := range rows {
		entries[i] = scoring.LeaderboardEntry{
			UserID:     row.ID,
			FirstName:  row.FirstName,
			Raw:        int(row.Raw),
			Normalized: row.Normalized,
			Rank:       i + 1,
		}
	}
	return entries, nil
}
