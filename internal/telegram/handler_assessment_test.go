package telegram_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleAssess_StartsAssessment(t *testing.T) {
	env := setupTestEnv(t)

	// Register user
	env.sendUpdate(t, makeTextUpdate(1, 700, 700, "alice", "Alice", "/start"))
	env.resetWireMock(t)

	// Start assessment
	env.sendUpdate(t, makeTextUpdate(2, 700, 700, "alice", "Alice", "/assess"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Вопрос 1/15")
	assert.Contains(t, lastReq, "inline_keyboard")
}

func TestHandleAssess_AlreadyOnboarded(t *testing.T) {
	env := setupTestEnv(t)

	// Register and onboard user
	u, err := env.userSvc.Register(context.Background(), 701, "bob", "Bob")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B2")
	require.NoError(t, err)

	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(1, 701, 701, "bob", "Bob", "/assess"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "уже прошёл")
	assert.Contains(t, lastReq, "B2")
}

func TestHandleAssess_FullFlow(t *testing.T) {
	env := setupTestEnv(t)

	// Register user
	env.sendUpdate(t, makeTextUpdate(1, 702, 702, "charlie", "Charlie", "/start"))

	// Start assessment
	env.sendUpdate(t, makeTextUpdate(2, 702, 702, "charlie", "Charlie", "/assess"))

	// Submit 30 answers via callback queries (always pick option 0)
	for i := 0; i < 30; i++ {
		env.sendUpdate(t, makeCallbackUpdate(int64(100+i), 702, 702, "charlie", "Charlie", "assess:0"))
	}

	// Verify user is now onboarded with a determined level
	u, err := env.userSvc.GetByTelegramID(context.Background(), 702)
	require.NoError(t, err)
	assert.True(t, u.Onboarded)
	assert.NotEmpty(t, u.Level)

	// Verify the final message contains the level
	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Тест завершён")
	assert.Contains(t, lastReq, string(u.Level))
}

func TestHandleAssess_ResumeSession(t *testing.T) {
	env := setupTestEnv(t)

	// Register user
	env.sendUpdate(t, makeTextUpdate(1, 703, 703, "diana", "Diana", "/start"))

	// Start assessment
	env.sendUpdate(t, makeTextUpdate(2, 703, 703, "diana", "Diana", "/assess"))

	// Answer 3 questions
	for i := 0; i < 3; i++ {
		env.sendUpdate(t, makeCallbackUpdate(int64(100+i), 703, 703, "diana", "Diana", "assess:1"))
	}

	env.resetWireMock(t)

	// Start again — should resume existing session
	env.sendUpdate(t, makeTextUpdate(3, 703, 703, "diana", "Diana", "/assess"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	// Should show question 4 (resuming after 3 answered, still in vocab phase)
	assert.Contains(t, lastReq, "Вопрос 4/15")
}
