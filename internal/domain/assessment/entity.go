package assessment

import (
	"time"

	"makeMe/internal/domain/user"
)

type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusCompleted  SessionStatus = "completed"
)

type Session struct {
	ID              int64
	UserID          int64
	Status          SessionStatus
	CurrentLevel    user.Level
	DeterminedLevel user.Level
	// PendingQuestion holds the question currently shown to the user so that
	// SubmitAnswer can check correctness against the right question.
	PendingQuestion    *Question
	ConsecutiveCorrect int
	CreatedAt          time.Time
	CompletedAt        time.Time
}

type Answer struct {
	ID            int64
	SessionID     int64
	QuestionLevel user.Level
	QuestionText  string
	Options       []string
	CorrectOption int
	UserOption    int
	Correct       bool
	Skipped       bool
	CreatedAt     time.Time
}

// Question represents a single assessment question from the question bank.
type Question struct {
	Level         user.Level
	Text          string
	Options       []string
	CorrectOption int // 0-based index
}
