package telegram_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleNewGroup(t *testing.T) {
	env := setupTestEnv(t)

	// Register user first
	env.sendUpdate(t, makeTextUpdate(1, 100, 100, "alice", "Alice", "/start"))
	env.resetWireMock(t)

	// Create group
	env.sendUpdate(t, makeTextUpdate(2, 100, 100, "alice", "Alice", "/newgroup English Club"))

	// Verify group was created in DB
	u, err := env.userSvc.GetByTelegramID(context.Background(), 100)
	require.NoError(t, err)

	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "English Club", groups[0].Name)
	assert.NotEmpty(t, groups[0].InviteCode)

	// Verify bot response contains invite code
	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "English Club")
	assert.Contains(t, lastReq, groups[0].InviteCode)
}

func TestHandleNewGroup_EmptyName(t *testing.T) {
	env := setupTestEnv(t)

	env.sendUpdate(t, makeTextUpdate(1, 101, 101, "bob", "Bob", "/start"))
	env.resetWireMock(t)

	// Send /newgroup without a name (just the prefix with trailing space)
	env.sendUpdate(t, makeTextUpdate(2, 101, 101, "bob", "Bob", "/newgroup "))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "/newgroup")
}

func TestHandleJoin_Success(t *testing.T) {
	env := setupTestEnv(t)

	// User 1 creates a group
	env.sendUpdate(t, makeTextUpdate(1, 200, 200, "alice", "Alice", "/start"))
	env.sendUpdate(t, makeTextUpdate(2, 200, 200, "alice", "Alice", "/newgroup Study Buddies"))

	u1, err := env.userSvc.GetByTelegramID(context.Background(), 200)
	require.NoError(t, err)
	groups, err := env.groupSvc.ListByUser(context.Background(), u1.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	inviteCode := groups[0].InviteCode

	// User 2 joins
	env.sendUpdate(t, makeTextUpdate(3, 201, 201, "bob", "Bob", "/start"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(4, 201, 201, "bob", "Bob", "/join "+inviteCode))

	// Verify user 2 is in the group
	u2, err := env.userSvc.GetByTelegramID(context.Background(), 201)
	require.NoError(t, err)
	u2Groups, err := env.groupSvc.ListByUser(context.Background(), u2.ID)
	require.NoError(t, err)
	require.Len(t, u2Groups, 1)
	assert.Equal(t, "Study Buddies", u2Groups[0].Name)

	// Verify bot response
	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Study Buddies")
}

func TestHandleJoin_InvalidCode(t *testing.T) {
	env := setupTestEnv(t)

	env.sendUpdate(t, makeTextUpdate(1, 300, 300, "charlie", "Charlie", "/start"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 300, 300, "charlie", "Charlie", "/join invalidcode"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "не найдена")
}

func TestHandleJoin_AlreadyMember(t *testing.T) {
	env := setupTestEnv(t)

	// Create group
	env.sendUpdate(t, makeTextUpdate(1, 400, 400, "alice", "Alice", "/start"))
	env.sendUpdate(t, makeTextUpdate(2, 400, 400, "alice", "Alice", "/newgroup Dupes Test"))

	u, err := env.userSvc.GetByTelegramID(context.Background(), 400)
	require.NoError(t, err)
	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	inviteCode := groups[0].InviteCode

	env.resetWireMock(t)

	// Try to join own group again
	env.sendUpdate(t, makeTextUpdate(3, 400, 400, "alice", "Alice", "/join "+inviteCode))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "уже состоишь")
}

func TestHandleMyGroups_Empty(t *testing.T) {
	env := setupTestEnv(t)

	env.sendUpdate(t, makeTextUpdate(1, 500, 500, "eve", "Eve", "/start"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 500, 500, "eve", "Eve", "/mygroups"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "нет групп")
}

func TestHandleMyGroups_WithGroups(t *testing.T) {
	env := setupTestEnv(t)

	// Create user and two groups
	env.sendUpdate(t, makeTextUpdate(1, 600, 600, "frank", "Frank", "/start"))
	env.sendUpdate(t, makeTextUpdate(2, 600, 600, "frank", "Frank", "/newgroup Group One"))
	env.sendUpdate(t, makeTextUpdate(3, 600, 600, "frank", "Frank", "/newgroup Group Two"))

	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(4, 600, 600, "frank", "Frank", "/mygroups"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Group One")
	assert.Contains(t, lastReq, "Group Two")
}
