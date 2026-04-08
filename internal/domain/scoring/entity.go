package scoring

type Score struct {
	ID         int64
	UserID     int64
	GroupID    int64
	Period     string // "day" | "month"
	PeriodKey  string // "2026-04-06" | "2026-04"
	Raw        int
	Normalized float64
}

type LeaderboardEntry struct {
	UserID     int64
	FirstName  string
	Raw        int
	Normalized float64
	Rank       int
}
