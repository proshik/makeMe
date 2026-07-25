package scoring

import (
	"testing"
	"time"
)

func day(t time.Time, offset int) string {
	return t.AddDate(0, 0, offset).Format("2006-01-02")
}

func TestStreak(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		days []string
		want int
	}{
		{"no history", nil, 0},
		{"today only", []string{day(now, 0)}, 1},
		{"five in a row ending today", []string{
			day(now, 0), day(now, -1), day(now, -2), day(now, -3), day(now, -4),
		}, 5},
		{
			// Documented behaviour, shared with the bot: the streak counts
			// today, so it reads 0 until the user practises on the current day.
			"today missing",
			[]string{day(now, -1), day(now, -2)},
			0,
		},
		{"gap in the middle", []string{day(now, 0), day(now, -1), day(now, -3)}, 2},
		{"unsorted input", []string{day(now, -2), day(now, 0), day(now, -1)}, 3},
		{"duplicates", []string{day(now, 0), day(now, 0), day(now, -1)}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Streak(tc.days, now); got != tc.want {
				t.Fatalf("Streak = %d, want %d", got, tc.want)
			}
		})
	}
}
