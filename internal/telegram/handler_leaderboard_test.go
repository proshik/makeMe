package telegram_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

func TestHandleLeaderboard_OneGroup(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 921, "lbuser1", "LBUser1")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 921, 921, "lbuser1", "LBUser1", "/newgroup MyGroup"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(2, 921, 921, "lbuser1", "LBUser1", "/leaderboard"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	last := reqs[len(reqs)-1].Body
	assert.Contains(t, last, "MyGroup")
	assert.Contains(t, last, "LBUser1")
}

func TestHandleLeaderboard_MultipleGroups(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 922, "lbuser2", "LBUser2")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 922, 922, "lbuser2", "LBUser2", "/newgroup Alpha"))
	env.sendUpdate(t, makeTextUpdate(2, 922, 922, "lbuser2", "LBUser2", "/newgroup Beta"))
	env.resetWireMock(t)

	env.sendUpdate(t, makeTextUpdate(3, 922, 922, "lbuser2", "LBUser2", "/leaderboard"))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	last := reqs[len(reqs)-1].Body
	assert.Contains(t, last, "Alpha")
	assert.Contains(t, last, "Beta")
	assert.Contains(t, last, "inline_keyboard")
}

func TestHandleLeaderboard_Callback(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 923, "lbuser3", "LBUser3")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "B1")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 923, 923, "lbuser3", "LBUser3", "/newgroup ScoreGroup"))

	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, groups)
	groupID := groups[0].ID

	err = env.scoringSvc.RecordScore(context.Background(), u.ID, groupID, user.B1, 10)
	require.NoError(t, err)

	env.resetWireMock(t)

	periodKey := time.Now().Format("2006-01-02")
	env.sendUpdate(t, makeCallbackUpdate(923, 923, 923, "lbuser3", "LBUser3",
		fmt.Sprintf("lb:%d:day:%s", groupID, periodKey)))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	last := reqs[len(reqs)-1].Body
	assert.Contains(t, last, "LBUser3")
	assert.Contains(t, last, "очков")
}

func TestHandleLeaderboard_MonthSwitch(t *testing.T) {
	env := setupTestEnv(t)

	u, err := env.userSvc.Register(context.Background(), 924, "lbuser4", "LBUser4")
	require.NoError(t, err)
	err = env.userSvc.UpdateLevel(context.Background(), u.ID, "A2")
	require.NoError(t, err)

	env.sendUpdate(t, makeTextUpdate(1, 924, 924, "lbuser4", "LBUser4", "/newgroup ToggleGroup"))

	groups, err := env.groupSvc.ListByUser(context.Background(), u.ID)
	require.NoError(t, err)
	groupID := groups[0].ID

	env.resetWireMock(t)

	periodKey := time.Now().Format("2006-01-02")
	env.sendUpdate(t, makeCallbackUpdate(924, 924, 924, "lbuser4", "LBUser4",
		fmt.Sprintf("lb:%d:day:%s", groupID, periodKey)))

	reqs := env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	dayResp := reqs[len(reqs)-1].Body
	assert.Contains(t, dayResp, "месяц")

	env.resetWireMock(t)

	monthKey := time.Now().Format("2006-01")
	env.sendUpdate(t, makeCallbackUpdate(925, 924, 924, "lbuser4", "LBUser4",
		fmt.Sprintf("lb:%d:month:%s", groupID, monthKey)))

	reqs = env.getSendMessageRequests(t)
	require.NotEmpty(t, reqs)
	monthResp := reqs[len(reqs)-1].Body
	assert.Contains(t, monthResp, "сегодня")
}
