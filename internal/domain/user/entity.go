package user

import "time"

type Level string

const (
	A1 Level = "A1"
	A2 Level = "A2"
	B1 Level = "B1"
	B2 Level = "B2"
	C1 Level = "C1"
	C2 Level = "C2"
)

// orderedLevels is the CEFR ladder from lowest to highest. Order is meaningful:
// it drives level promotion, word selection and every A1→C2 progress display.
var orderedLevels = []Level{A1, A2, B1, B2, C1, C2}

// AllLevels returns the CEFR ladder in ascending order. The returned slice is a
// copy, so callers cannot reorder the canonical one.
func AllLevels() []Level {
	out := make([]Level, len(orderedLevels))
	copy(out, orderedLevels)
	return out
}

// Index returns the position of a level on the ladder, or -1 if unknown.
func (l Level) Index() int {
	for i, v := range orderedLevels {
		if v == l {
			return i
		}
	}
	return -1
}

// AtLeast reports whether l is at or above other. An unknown level is below
// everything, so an unset user level never lights up a progress scale.
func (l Level) AtLeast(other Level) bool {
	li, oi := l.Index(), other.Index()
	if li < 0 || oi < 0 {
		return false
	}
	return li >= oi
}

type User struct {
	ID         int64
	TelegramID int64
	Username   string
	FirstName  string
	Level      Level
	Onboarded  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
