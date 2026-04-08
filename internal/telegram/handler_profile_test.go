package telegram_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
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
	assert.Contains(t, body, "Выучено слов")
}

