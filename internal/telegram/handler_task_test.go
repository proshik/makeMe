package telegram_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleTasks_NotOnboarded(t *testing.T) {
	env := setupTestEnv(t)

	env.sendUpdate(t, makeTextUpdate(1, 800, 800, "alice", "Alice", "/start"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 800, 800, "alice", "Alice", "/tasks"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "/assess")
}

func TestHandleTasks_NoGroups_AutoCreatesSoloGroup(t *testing.T) {
	env := setupTestEnv(t)

	// Register and onboard user (no groups)
	u, err := env.userSvc.Register(context.Background(), 801, "bob", "Bob")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(1, 801, 801, "bob", "Bob", "/tasks"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	// Should auto-create solo group and show first task (not an error)
	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "Задание 1/")
	assert.Contains(t, lastReq, "inline_keyboard")
}

func TestHandleTasks_GeneratesAndShowsTask(t *testing.T) {
	env := setupTestEnv(t)

	// Register, onboard, create group
	u, err := env.userSvc.Register(context.Background(), 802, "charlie", "Charlie")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "A1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 802, 802, "charlie", "Charlie", "/newgroup Test Group"))
	env.resetWireMock(t)

	// Request tasks
	env.sendUpdate(t, makeTextUpdate(2, 802, 802, "charlie", "Charlie", "/tasks"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	// Should show a task with inline keyboard
	assert.Contains(t, lastReq, "Задание 1/")
	assert.Contains(t, lastReq, "inline_keyboard")
}

func TestHandleTasks_FullFlow(t *testing.T) {
	env := setupTestEnv(t)

	// Setup: register, onboard, create group
	u, err := env.userSvc.Register(context.Background(), 803, "diana", "Diana")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "A1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 803, 803, "diana", "Diana", "/newgroup Study"))

	// Get group
	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, groups)

	// Generate tasks
	ds, err := env.taskSvc.GenerateDaily(context.Background(), u.ID, groups[0].ID, u.Level)
	require.NoError(t, err)
	require.NotEmpty(t, ds.Tasks)

	env.resetWireMock(t)

	// Answer all tasks via callbacks
	for _, task := range ds.Tasks {
		// Find correct option index
		correctIdx := 0
		for i, opt := range task.Options {
			if opt == task.Answer {
				correctIdx = i
				break
			}
		}
		env.sendUpdate(t, makeCallbackUpdate(int64(200+task.ID), 803, 803, "diana", "Diana",
			"task:"+itoa(task.ID)+":"+itoa(int64(correctIdx))))
	}

	// Check completion message
	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "выполнены")
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}

func TestHandleTasks_ShowsRoundHeader_OnFirstRound(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 820, "erin", "Erin")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "A1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 820, 820, "erin", "Erin", "/newgroup Round"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 820, 820, "erin", "Erin", "/tasks"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)

	// The bot must announce the current round out of maxRounds (5 by default)
	// as a standalone message separate from the task question.
	found := false
	for _, r := range reqs {
		if strings.Contains(r.Body, "Раунд 1 из 5") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a message containing 'Раунд 1 из 5', got: %+v", reqs)
}

func TestHandleTasks_ShowsLimitMessage_WhenRoundsExhausted(t *testing.T) {
	env := setupTestEnvWithMaxRounds(t, 1)

	u, err := env.userSvc.Register(context.Background(), 821, "frank", "Frank")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "A1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 821, 821, "frank", "Frank", "/newgroup Cap"))
	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, groups)

	ds, err := env.taskSvc.GenerateDaily(context.Background(), u.ID, groups[0].ID, u.Level)
	require.NoError(t, err)
	require.NotEmpty(t, ds.Tasks)

	// Answer all 10 tasks in round 0
	for _, tt := range ds.Tasks {
		correctIdx := 0
		for i, opt := range tt.Options {
			if opt == tt.Answer {
				correctIdx = i
				break
			}
		}
		env.sendUpdate(t, makeCallbackUpdate(int64(300+tt.ID), 821, 821, "frank", "Frank",
			"task:"+itoa(tt.ID)+":"+itoa(int64(correctIdx))))
	}

	env.resetWireMock(t)

	// With maxRounds=1, the second /tasks must yield the limit message.
	env.sendUpdate(t, makeTextUpdate(99, 821, 821, "frank", "Frank", "/tasks"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	lastReq := reqs[len(reqs)-1].Body
	assert.Contains(t, lastReq, "раунд")
	assert.Contains(t, lastReq, "завтра")
	// No new task must be sent when the limit is hit.
	assert.NotContains(t, lastReq, "inline_keyboard")
}

func TestHandleWordAudio_WithCachedAudio(t *testing.T) {
	env := setupTestEnv(t)

	// Register user
	_, err := env.userSvc.Register(context.Background(), 910, "audiouser", "AudioUser")
	require.NoError(t, err)

	// Get a seeded word and pre-populate its audio URL
	words, err := env.srsRepo.GetWordsByLevel(context.Background(), "A1", 1)
	require.NoError(t, err)
	require.NotEmpty(t, words)
	word := words[0]

	err = env.srsRepo.UpdateWordAudio(context.Background(), word.ID, "/test/", "https://example.com/audio.mp3")
	require.NoError(t, err)

	env.resetWireMock(t)

	// Send word_audio callback
	env.sendUpdate(t, makeCallbackUpdate(910, 910, 910, "audiouser", "AudioUser",
		fmt.Sprintf("word_audio:%d", word.ID)))

	// Verify sendAudio was called with the cached URL
	reqs := env.getRequests(t, "/bottest-token/sendAudio")
	require.NotEmpty(t, reqs)
	assert.Contains(t, reqs[0].Body, "example.com/audio.mp3")
}

func TestHandleWordAudio_NoAudioURL(t *testing.T) {
	env := setupTestEnv(t)

	// Register user
	_, err := env.userSvc.Register(context.Background(), 911, "noaudio", "NoAudio")
	require.NoError(t, err)

	// Get a seeded word — it has no audio_url by default
	words, err := env.srsRepo.GetWordsByLevel(context.Background(), "A1", 1)
	require.NoError(t, err)
	require.NotEmpty(t, words)
	word := words[0]
	// Ensure no audio URL is set
	err = env.srsRepo.UpdateWordAudio(context.Background(), word.ID, "", "")
	require.NoError(t, err)

	env.resetWireMock(t)

	// Send word_audio callback; dictClient may attempt lookup but word has no audio
	// The handler should send a "not found" message if no audio is available
	env.sendUpdate(t, makeCallbackUpdate(911, 911, 911, "noaudio", "NoAudio",
		fmt.Sprintf("word_audio:%d", word.ID)))

	// Either sendAudio (if dictClient fetched audio) or sendMessage with "not found"
	audioReqs := env.getRequests(t, "/bottest-token/sendAudio")
	msgReqs := env.getSendMessageRequests(t)
	assert.True(t, len(audioReqs) > 0 || len(msgReqs) > 0, "handler should respond with audio or not-found message")
}
