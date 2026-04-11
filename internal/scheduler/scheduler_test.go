package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextRunTime_BeforeTarget_ReturnsSameDay(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	// 15:00 MSK — before 21:00 target
	now := time.Date(2026, 4, 11, 15, 0, 0, 0, loc)
	target := nextRunTime(now, "21:00", loc)

	assert.Equal(t, 2026, target.Year())
	assert.Equal(t, time.April, target.Month())
	assert.Equal(t, 11, target.Day())
	assert.Equal(t, 21, target.Hour())
	assert.Equal(t, 0, target.Minute())
	assert.Equal(t, 0, target.Second())
}

func TestNextRunTime_AfterTarget_ReturnsTomorrow(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	// 22:30 MSK — after 21:00 target → should be tomorrow
	now := time.Date(2026, 4, 11, 22, 30, 0, 0, loc)
	target := nextRunTime(now, "21:00", loc)

	assert.Equal(t, 12, target.Day())
	assert.Equal(t, 21, target.Hour())
	assert.Equal(t, 0, target.Minute())
}

func TestNextRunTime_ExactTarget_ReturnsTomorrow(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	// Exactly 21:00 — already at target, should return tomorrow
	now := time.Date(2026, 4, 11, 21, 0, 0, 0, loc)
	target := nextRunTime(now, "21:00", loc)

	assert.Equal(t, 12, target.Day())
	assert.Equal(t, 21, target.Hour())
}

func TestNextRunTime_MidnightEdge(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	// 00:01 MSK — well before 21:00, should be today
	now := time.Date(2026, 4, 11, 0, 1, 0, 0, loc)
	target := nextRunTime(now, "21:00", loc)

	assert.Equal(t, 11, target.Day())
	assert.Equal(t, 21, target.Hour())
}

func TestFormatDailySummary_WithScores(t *testing.T) {
	summaries := []groupSummary{
		{GroupName: "Study Crew", Raw: 100, Normalized: 115.0, Rank: 1, Total: 3},
	}

	msg := formatDailySummary("Alice", summaries)

	assert.Contains(t, msg, "Alice")
	assert.Contains(t, msg, "Study Crew")
	assert.Contains(t, msg, "100")
	assert.Contains(t, msg, "1 из 3")
}

func TestFormatDailySummary_MultipleGroups(t *testing.T) {
	summaries := []groupSummary{
		{GroupName: "Team A", Raw: 80, Normalized: 80.0, Rank: 2, Total: 5},
		{GroupName: "Team B", Raw: 50, Normalized: 57.5, Rank: 1, Total: 2},
	}

	msg := formatDailySummary("Bob", summaries)

	assert.Contains(t, msg, "Team A")
	assert.Contains(t, msg, "Team B")
	assert.Contains(t, msg, "2 из 5")
	assert.Contains(t, msg, "1 из 2")
}

func TestFormatDailySummary_NoScores_PromptsToDoTasks(t *testing.T) {
	msg := formatDailySummary("Carlos", nil)

	assert.Contains(t, msg, "Carlos")
	assert.Contains(t, msg, "/tasks")
}

func TestFormatReassessReminder_ContainsAssessCommand(t *testing.T) {
	msg := formatReassessReminder("Diana")

	assert.Contains(t, msg, "Diana")
	assert.Contains(t, msg, "/assess")
}
