package telegram_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleStart_NewUser(t *testing.T) {
	env := setupTestEnv(t)

	env.sendUpdate(t, makeTextUpdate(1, 111, 111, "alice", "Alice", "/start"))

	// Verify user was created in DB
	u, err := env.userSvc.GetByTelegramID(context.Background(), 111)
	require.NoError(t, err)
	assert.Equal(t, int64(111), u.TelegramID)
	assert.Equal(t, "alice", u.Username)
	assert.Equal(t, "Alice", u.FirstName)
	assert.False(t, u.Onboarded)

	// Verify bot sent a welcome message
	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Alice")
	assert.Contains(t, lastReq, "makeMe")
}

func TestHandleStart_ReturningUser(t *testing.T) {
	env := setupTestEnv(t)

	// Register and onboard the user
	u, err := env.userSvc.Register(context.Background(), 222, "bob", "Bob")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B2")
	require.NoError(t, err)

	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 222, 222, "bob", "Bob", "/start"))

	// Verify welcome back message with level
	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Bob")
	assert.Contains(t, lastReq, "B2")
}

func TestHandleStart_UpdatesUsername(t *testing.T) {
	env := setupTestEnv(t)

	// First registration
	_, err := env.userSvc.Register(context.Background(), 333, "old_name", "Charlie")
	require.NoError(t, err)

	// Send /start with new username
	env.sendUpdate(t, makeTextUpdate(3, 333, 333, "new_name", "Charlie", "/start"))

	// Verify username was updated (Create is upsert)
	u, err := env.userSvc.GetByTelegramID(context.Background(), 333)
	require.NoError(t, err)
	assert.Equal(t, "new_name", u.Username)
}

func TestHandleUnknownCommand(t *testing.T) {
	env := setupTestEnv(t)

	env.sendUpdate(t, makeTextUpdate(4, 444, 444, "dave", "Dave", "/unknown"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.True(t, strings.Contains(lastReq, "/start") || strings.Contains(lastReq, "Неизвестная"))
}
