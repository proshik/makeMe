package telegram_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
	"makeMe/internal/storage/sqlc"
)

func TestHandleProfile_NotOnboarded(t *testing.T) {
	env := setupTestEnv(t)

	_, err := env.userSvc.Register(context.Background(), 930, "profuser0", "ProfUser0")
	require.NoError(t, err)
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(1, 930, 930, "profuser0", "ProfUser0", "/profile"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	assert.Contains(t, reqs[len(reqs)-1].Body, "/assess")
}

func TestHandleProfile_Onboarded(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 931, "profuser1", "ProfUser1")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 931, 931, "profuser1", "ProfUser1", "/newgroup ProfGroup"))
	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, groups)

	err = env.scoringSvc.RecordScore(context.Background(), u.ID, groups[0].ID, user.B1, 10)
	require.NoError(t, err)

	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 931, 931, "profuser1", "ProfUser1", "/profile"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	body := reqs[len(reqs)-1].Body
	assert.Contains(t, body, "B1")
	assert.Contains(t, body, "баллов")
	assert.Contains(t, body, "Дней подряд")
	assert.Contains(t, body, "интервальных повторениях")
}

func TestHandleProfile_LevelProgressBlock(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	u, err := env.userSvc.Register(ctx, 932, "profuser2", "ProfUser2")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(ctx, u.ID, "A1")
	require.NoError(t, err)

	// Migration 00006_seed_words populates the `words` table. Look up the
	// actual A1 corpus size, then mark 2 of those as already-known so the
	// per-level block shows non-zero progress for A1.
	a1Total, err := env.queries.GetWordsByLevel(ctx, sqlc.GetWordsByLevelParams{Level: "A1", Limit: 1000})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(a1Total), 2, "test relies on at least 2 seeded A1 words")

	for _, w := range a1Total[:2] {
		require.NoError(t, env.wordsRepo.MarkAlreadyKnown(ctx, u.ID, w.ID))
	}

	env.resetWireMock(t)
	env.sendUpdate(t, makeTextUpdate(1, 932, 932, "profuser2", "ProfUser2", "/profile"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	body := reqs[len(reqs)-1].Body

	assert.Contains(t, body, "По уровням:")
	expectedA1 := fmt.Sprintf("A1: 2/%d (", len(a1Total))
	assert.Contains(t, body, expectedA1, "A1 progress line missing/wrong count")
	// Levels the user hasn't started yet (Learned = 0) must be hidden so the
	// block stays focused on actual progress.
	assert.NotContains(t, body, "A2: 0/")
	assert.NotContains(t, body, "B1: 0/")
	assert.NotContains(t, body, "C1: 0/")

	// Section header for the words group should appear above the per-level block.
	assert.Contains(t, body, "📚 Слова")
}

