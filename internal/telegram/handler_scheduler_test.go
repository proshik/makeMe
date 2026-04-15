package telegram_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/scheduler"
)

func newSched(e *testEnv) *scheduler.Scheduler {
	return scheduler.New(
		e.userRepo,
		e.groupSvc,
		e.scoringSvc,
		e.assessRepo,
		e.tgBot.SendMessage,
		time.UTC,
		"21:00",
	)
}

// TestScheduler_RunDailySummary_NoGroups verifies that a user with no groups receives no message.
func TestScheduler_RunDailySummary_NoGroups(t *testing.T) {
	e := setupTestEnv(t)
	ctx := context.Background()

	// Register user but do NOT join any group
	e.sendUpdate(t, makeTextUpdate(1, 111, 111, "loner", "Loner", "/start"))

	// Record baseline — any requests from setup must not be counted as scheduler output
	baseline := len(e.getSendMessageRequests(t))

	err := newSched(e).RunDailySummary(ctx)
	require.NoError(t, err)

	newMsgs := e.getSendMessageRequests(t)[baseline:]
	assert.Empty(t, newMsgs, "user without groups should not receive a daily summary")
}

// TestScheduler_RunDailySummary_WithNoScoresToday verifies that a user in a group
// but with no tasks done today receives the "no tasks done" prompt.
func TestScheduler_RunDailySummary_WithNoScoresToday(t *testing.T) {
	e := setupTestEnv(t)
	ctx := context.Background()

	// Register user and create a group
	e.sendUpdate(t, makeTextUpdate(2, 222, 222, "user2", "Alexei", "/start"))
	e.sendUpdate(t, makeTextUpdate(3, 222, 222, "user2", "Alexei", "/newgroup TestGroup"))

	// Record baseline — any requests from setup must not be counted as scheduler output
	baseline := len(e.getSendMessageRequests(t))

	err := newSched(e).RunDailySummary(ctx)
	require.NoError(t, err)

	// User should receive a "no tasks today" message
	newMsgs := e.getSendMessageRequests(t)[baseline:]
	require.Len(t, newMsgs, 1, "user in a group should receive a daily summary")

	// The bot sends multipart form-data; use Contains instead of JSON unmarshaling
	body := newMsgs[0].Body
	assert.Contains(t, body, "Alexei")
	assert.Contains(t, body, "/tasks")
}
