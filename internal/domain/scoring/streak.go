package scoring

import "time"

// Streak counts consecutive scoring days ending today. A day with no score
// ends the count — including today, so the streak reads 0 until the user has
// practised on the current day.
//
// That last part is deliberate and matches what the bot has always shown: the
// streak is a claim about "today included", not about the run so far. It does
// mean a user with a long run sees 0 every morning until they practise; if
// that should change, it must change in both channels at once, which is why
// this lives in the domain rather than in an adapter.
func Streak(days []string, now time.Time) int {
	set := make(map[string]bool, len(days))
	for _, d := range days {
		set[d] = true
	}

	streak := 0
	for d := now; set[d.Format("2006-01-02")]; d = d.AddDate(0, 0, -1) {
		streak++
	}
	return streak
}
