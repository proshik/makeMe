package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
	"makeMe/internal/storage/sqlc"
)

type WordsRepository struct {
	q *sqlc.Queries
}

func NewWordsRepository(q *sqlc.Queries) *WordsRepository {
	return &WordsRepository{q: q}
}

// --- Word selection ---

func (r *WordsRepository) GetInProgressWordsByLevel(ctx context.Context, userID int64, level user.Level, limit int) ([]srs.Word, error) {
	rows, err := r.q.GetInProgressWordsByLevel(ctx, sqlc.GetInProgressWordsByLevelParams{
		UserID: userID,
		Level:  string(level),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get in-progress words: %w", err)
	}
	out := make([]srs.Word, len(rows))
	for i, row := range rows {
		out[i] = srs.Word{
			ID:            row.ID,
			English:       row.English,
			Russian:       row.Russian,
			Level:         user.Level(row.Level),
			Example:       row.Example,
			Transcription: row.Transcription,
			AudioURL:      row.AudioUrl,
		}
	}
	return out, nil
}

func (r *WordsRepository) GetNewWordsExcludingProgress(ctx context.Context, userID int64, level user.Level, limit int) ([]srs.Word, error) {
	rows, err := r.q.GetNewWordsExcludingProgress(ctx, sqlc.GetNewWordsExcludingProgressParams{
		UserID: userID,
		Level:  string(level),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get new words excluding progress: %w", err)
	}
	out := make([]srs.Word, len(rows))
	for i, row := range rows {
		out[i] = srs.Word{
			ID:            row.ID,
			English:       row.English,
			Russian:       row.Russian,
			Level:         user.Level(row.Level),
			Example:       row.Example,
			Transcription: row.Transcription,
			AudioURL:      row.AudioUrl,
		}
	}
	return out, nil
}

// --- Progress ---

func (r *WordsRepository) GetProgress(ctx context.Context, userID, wordID int64) (*words.Progress, error) {
	row, err := r.q.GetWordProgress(ctx, sqlc.GetWordProgressParams{
		UserID: userID,
		WordID: wordID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get word progress: %w", err)
	}
	p := toProgress(row)
	return &p, nil
}

func (r *WordsRepository) BulkGetProgress(ctx context.Context, userID int64, wordIDs []int64) (map[int64]words.Progress, error) {
	rows, err := r.q.BulkGetProgressForWords(ctx, sqlc.BulkGetProgressForWordsParams{
		UserID:  userID,
		Column2: wordIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("bulk get progress: %w", err)
	}
	out := make(map[int64]words.Progress, len(rows))
	for _, row := range rows {
		out[row.WordID] = toProgress(row)
	}
	return out, nil
}

func (r *WordsRepository) IncProgress(ctx context.Context, userID, wordID int64, mode words.Mode) error {
	switch mode {
	case words.ModeEnRuMC:
		return r.q.IncEnRuMC(ctx, sqlc.IncEnRuMCParams{UserID: userID, WordID: wordID})
	case words.ModeRuEnMC:
		return r.q.IncRuEnMC(ctx, sqlc.IncRuEnMCParams{UserID: userID, WordID: wordID})
	case words.ModeEnRuTyping:
		return r.q.IncEnRuTyping(ctx, sqlc.IncEnRuTypingParams{UserID: userID, WordID: wordID})
	case words.ModeRuEnTyping:
		return r.q.IncRuEnTyping(ctx, sqlc.IncRuEnTypingParams{UserID: userID, WordID: wordID})
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

func (r *WordsRepository) ResetProgress(ctx context.Context, userID, wordID int64, mode words.Mode) error {
	switch mode {
	case words.ModeEnRuMC:
		return r.q.ResetEnRuMC(ctx, sqlc.ResetEnRuMCParams{UserID: userID, WordID: wordID})
	case words.ModeRuEnMC:
		return r.q.ResetRuEnMC(ctx, sqlc.ResetRuEnMCParams{UserID: userID, WordID: wordID})
	case words.ModeEnRuTyping:
		return r.q.ResetEnRuTyping(ctx, sqlc.ResetEnRuTypingParams{UserID: userID, WordID: wordID})
	case words.ModeRuEnTyping:
		return r.q.ResetRuEnTyping(ctx, sqlc.ResetRuEnTypingParams{UserID: userID, WordID: wordID})
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

func (r *WordsRepository) MarkLearned(ctx context.Context, userID, wordID int64) error {
	return r.q.MarkLearned(ctx, sqlc.MarkLearnedParams{UserID: userID, WordID: wordID})
}

func (r *WordsRepository) MarkAlreadyKnown(ctx context.Context, userID, wordID int64) error {
	return r.q.MarkAlreadyKnown(ctx, sqlc.MarkAlreadyKnownParams{UserID: userID, WordID: wordID})
}

func (r *WordsRepository) CountInProgress(ctx context.Context, userID int64) (int, error) {
	c, err := r.q.CountInProgressForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count in progress: %w", err)
	}
	return int(c), nil
}

func (r *WordsRepository) CountLearnedViaWords(ctx context.Context, userID int64) (int, error) {
	c, err := r.q.CountLearnedViaWordsForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count learned via words: %w", err)
	}
	return int(c), nil
}

func (r *WordsRepository) CountWordsByLevel(ctx context.Context, userID int64) ([]words.LevelProgress, error) {
	rows, err := r.q.CountWordsByLevelForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count words by level: %w", err)
	}
	out := make([]words.LevelProgress, 0, len(rows))
	for _, row := range rows {
		out = append(out, words.LevelProgress{
			Level:   row.Level,
			Learned: int(row.Learned),
			Total:   int(row.Total),
		})
	}
	return out, nil
}

// --- Sessions ---

func (r *WordsRepository) CreateSession(ctx context.Context, userID int64, startedOn time.Time, wordIDs []int64) (*words.Session, error) {
	row, err := r.q.CreateWordsSession(ctx, sqlc.CreateWordsSessionParams{
		UserID:    userID,
		StartedOn: pgtype.Date{Time: startedOn, Valid: true},
		WordIds:   wordIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	s := toSession(row)
	return &s, nil
}

func (r *WordsRepository) GetOpenSession(ctx context.Context, userID int64) (*words.Session, error) {
	row, err := r.q.GetOpenWordsSession(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get open session: %w", err)
	}
	s := toSession(row)
	return &s, nil
}

func (r *WordsRepository) GetSessionByID(ctx context.Context, id int64) (*words.Session, error) {
	row, err := r.q.GetWordsSessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session by id: %w", err)
	}
	s := toSession(row)
	return &s, nil
}

func (r *WordsRepository) UpdateLastWord(ctx context.Context, sessionID, wordID int64) error {
	return r.q.UpdateLastWord(ctx, sqlc.UpdateLastWordParams{
		ID:         sessionID,
		LastWordID: pgtype.Int8{Int64: wordID, Valid: wordID != 0},
	})
}

func (r *WordsRepository) CompleteSession(ctx context.Context, sessionID int64) error {
	return r.q.CompleteWordsSession(ctx, sessionID)
}

func (r *WordsRepository) IncSessionWordError(ctx context.Context, sessionID, wordID int64) error {
	return r.q.IncSessionWordError(ctx, sqlc.IncSessionWordErrorParams{
		ID:      sessionID,
		Column2: fmt.Sprintf("%d", wordID),
	})
}

func (r *WordsRepository) GetSessionWordError(ctx context.Context, sessionID, wordID int64) (int, error) {
	c, err := r.q.GetSessionWordError(ctx, sqlc.GetSessionWordErrorParams{
		ID:      sessionID,
		Column2: fmt.Sprintf("%d", wordID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get session word error: %w", err)
	}
	return int(c), nil
}

func (r *WordsRepository) DeferSessionWord(ctx context.Context, sessionID, wordID int64) error {
	return r.q.DeferSessionWord(ctx, sqlc.DeferSessionWordParams{
		ID:      sessionID,
		Column2: wordID,
	})
}

// --- helpers ---

func toProgress(row sqlc.WordLearningProgress) words.Progress {
	p := words.Progress{
		UserID: row.UserID,
		WordID: row.WordID,
		Counts: map[words.Mode]int{
			words.ModeEnRuMC:     int(row.EnRuMcCorrect),
			words.ModeRuEnMC:     int(row.RuEnMcCorrect),
			words.ModeEnRuTyping: int(row.EnRuTypingCorrect),
			words.ModeRuEnTyping: int(row.RuEnTypingCorrect),
		},
		Learned:   row.Learned,
		UpdatedAt: row.UpdatedAt.Time,
	}
	if row.LearnedAt.Valid {
		t := row.LearnedAt.Time
		p.LearnedAt = &t
	}
	return p
}

func toSession(row sqlc.WordsSession) words.Session {
	s := words.Session{
		ID:              row.ID,
		UserID:          row.UserID,
		StartedOn:       row.StartedOn.Time,
		WordIDs:         row.WordIds,
		DeferredWordIDs: row.DeferredWordIds,
	}
	if row.LastWordID.Valid {
		s.LastWordID = row.LastWordID.Int64
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		s.CompletedAt = &t
	}
	return s
}
