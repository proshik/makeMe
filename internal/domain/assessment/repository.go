package assessment

import "context"

type Repository interface {
	CreateSession(ctx context.Context, userID int64) (*Session, error)
	GetActiveSession(ctx context.Context, userID int64) (*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
	AddAnswer(ctx context.Context, answer *Answer) error
	CountAnswers(ctx context.Context, sessionID int64) (int, error)
	CountCorrectByLevel(ctx context.Context, sessionID int64) (map[string]LevelStats, error)
}

// LevelStats holds correct/total counts for a single level within a session.
type LevelStats struct {
	Correct int
	Total   int
}
