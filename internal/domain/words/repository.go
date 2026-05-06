package words

import (
	"context"
	"time"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

// Repository abstracts persistence for /words: per-mode progress, sessions,
// and word-selection queries that are specific to learning mode.
type Repository interface {
	// Word selection (joins words ↔ word_learning_progress / review_cards)
	GetInProgressWordsByLevel(ctx context.Context, userID int64, level user.Level, limit int) ([]srs.Word, error)
	GetNewWordsExcludingProgress(ctx context.Context, userID int64, level user.Level, limit int) ([]srs.Word, error)

	// Progress
	GetProgress(ctx context.Context, userID, wordID int64) (*Progress, error)
	BulkGetProgress(ctx context.Context, userID int64, wordIDs []int64) (map[int64]Progress, error)
	IncProgress(ctx context.Context, userID, wordID int64, mode Mode) error
	ResetProgress(ctx context.Context, userID, wordID int64, mode Mode) error
	MarkLearned(ctx context.Context, userID, wordID int64) error
	MarkAlreadyKnown(ctx context.Context, userID, wordID int64) error
	CountInProgress(ctx context.Context, userID int64) (int, error)
	CountLearnedViaWords(ctx context.Context, userID int64) (int, error)
	CountWordsByLevel(ctx context.Context, userID int64) ([]LevelProgress, error)

	// Sessions
	CreateSession(ctx context.Context, userID int64, startedOn time.Time, wordIDs []int64) (*Session, error)
	GetOpenSession(ctx context.Context, userID int64) (*Session, error)
	GetSessionByID(ctx context.Context, id int64) (*Session, error)
	UpdateLastWord(ctx context.Context, sessionID, wordID int64) error
	CompleteSession(ctx context.Context, sessionID int64) error

	// Per-session error tracking (drives the "defer until tomorrow" affordance)
	IncSessionWordError(ctx context.Context, sessionID, wordID int64) error
	GetSessionWordError(ctx context.Context, sessionID, wordID int64) (int, error)
	DeferSessionWord(ctx context.Context, sessionID, wordID int64) error
}

// PendingRepository abstracts the pending_typing_prompts table.
type PendingRepository interface {
	Upsert(ctx context.Context, p *PendingTyping) error
	Get(ctx context.Context, userID int64) (*PendingTyping, error)
	Delete(ctx context.Context, userID int64) error
}
