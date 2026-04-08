package telegram

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalcStreak_ConsecutiveDays(t *testing.T) {
	today := time.Now()
	days := []string{
		today.Format("2006-01-02"),
		today.AddDate(0, 0, -1).Format("2006-01-02"),
		today.AddDate(0, 0, -2).Format("2006-01-02"),
		today.AddDate(0, 0, -3).Format("2006-01-02"),
		today.AddDate(0, 0, -4).Format("2006-01-02"),
	}
	assert.Equal(t, 5, calcStreak(days))
}

func TestCalcStreak_TodayMissing(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	days := []string{
		yesterday.Format("2006-01-02"),
		yesterday.AddDate(0, 0, -1).Format("2006-01-02"),
	}
	assert.Equal(t, 0, calcStreak(days))
}

func TestCalcStreak_GapInMiddle(t *testing.T) {
	today := time.Now()
	days := []string{
		today.Format("2006-01-02"),
		today.AddDate(0, 0, -1).Format("2006-01-02"),
		// gap on -2
		today.AddDate(0, 0, -3).Format("2006-01-02"),
		today.AddDate(0, 0, -4).Format("2006-01-02"),
	}
	assert.Equal(t, 2, calcStreak(days))
}
