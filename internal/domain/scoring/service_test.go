package scoring

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

type fakeRepo struct {
	calls []upsertCall
}

type upsertCall struct {
	userID, groupID   int64
	period, periodKey string
	rawDelta          int
	normDelta         float64
}

func (f *fakeRepo) Upsert(_ context.Context, userID, groupID int64, period, periodKey string, rawDelta int, normDelta float64) error {
	f.calls = append(f.calls, upsertCall{userID, groupID, period, periodKey, rawDelta, normDelta})
	return nil
}

func (f *fakeRepo) GetLeaderboard(_ context.Context, _ int64, _, _ string) ([]LeaderboardEntry, error) {
	return nil, nil
}

var _ Repository = (*fakeRepo)(nil)

func TestNormalizeScore(t *testing.T) {
	cases := []struct {
		level    user.Level
		raw      int
		wantNorm float64
	}{
		{user.A1, 10, 6.0},
		{user.A2, 10, 8.0},
		{user.B1, 10, 10.0},
		{user.B2, 10, 11.5},
		{user.C1, 10, 13.0},
		{user.C2, 10, 15.0},
	}
	for _, tc := range cases {
		t.Run(string(tc.level), func(t *testing.T) {
			got := normalizeScore(tc.raw, tc.level)
			assert.Equal(t, tc.wantNorm, got)
		})
	}
}

func TestRecordScore_CallsUpsertForBothPeriods(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	err := svc.RecordScore(context.Background(), 1, 2, user.A1, 10)
	require.NoError(t, err)

	require.Len(t, repo.calls, 2, "should upsert day and month records")

	dayCall := repo.calls[0]
	assert.Equal(t, "day", dayCall.period)
	assert.Equal(t, time.Now().Format("2006-01-02"), dayCall.periodKey)
	assert.Equal(t, 10, dayCall.rawDelta)
	assert.Equal(t, 6.0, dayCall.normDelta)

	monthCall := repo.calls[1]
	assert.Equal(t, "month", monthCall.period)
	assert.Equal(t, time.Now().Format("2006-01"), monthCall.periodKey)
	assert.Equal(t, 10, monthCall.rawDelta)
	assert.Equal(t, 6.0, monthCall.normDelta)
}

func TestRecordScore_UnknownLevel_UsesOneAsCoefficient(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	err := svc.RecordScore(context.Background(), 1, 1, user.Level("XX"), 10)
	require.NoError(t, err)

	require.NotEmpty(t, repo.calls)
	assert.Equal(t, 10.0, repo.calls[0].normDelta)
}

func TestNormalizeScore_ZeroRaw(t *testing.T) {
	got := normalizeScore(0, user.C2)
	assert.Equal(t, 0.0, got)
}
