package words

import (
	"context"
	"errors"
	"fmt"
	"time"

	"makeMe/internal/domain/srs"
	"makeMe/internal/textmatch"
)

var (
	ErrSessionNotFound  = errors.New("words session not found")
	ErrWordNotInSession = errors.New("word not in session")
)

// StatusView is the lightweight summary shown when /words is invoked.
type StatusView struct {
	WordsInSession   int
	ModesCompleted   int             // sum of min(counts, 2) over all words+modes; max = WordsInSession*4*2
	ModesTotal       int             // WordsInSession * 4 * ThresholdPerMode
	LearnedAllTime   int             // life-time count from word_learning_progress
	LearnedThisRound int             // already learned among current session's word_ids
	ByLevel          []LevelProgress // per-CEFR-level learned/total; nil on query error
}

// Service is the high-level facade for /words mode.
type Service struct {
	repo    Repository
	srsRepo srs.Repository
	now     func() time.Time // injectable clock for tests
}

func NewService(repo Repository, srsRepo srs.Repository) *Service {
	return &Service{repo: repo, srsRepo: srsRepo, now: time.Now}
}

// StartOrResumeSession returns the open session (resume) or creates a new one
// with a fresh selection. There is no per-day limit: after CompleteSession
// the next call creates a new session with a new pick. The `resumed` return
// flag tells the caller whether an existing open session was returned.
//
// Word selection deliberately ignores the user's profile level — see
// SelectWords. The user climbs A1 → A2 → … → C2 by exhausting each level
// before the next one starts feeding into sessions.
func (s *Service) StartOrResumeSession(ctx context.Context, userID int64) (session *Session, ws []*srs.Word, resumed bool, err error) {
	if open, err := s.repo.GetOpenSession(ctx, userID); err != nil {
		return nil, nil, false, err
	} else if open != nil {
		ws, err := s.loadSessionWords(ctx, open.WordIDs)
		if err != nil {
			return nil, nil, false, err
		}
		return open, ws, true, nil
	}

	picked, err := SelectWords(ctx, s.repo, userID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("select words: %w", err)
	}
	if len(picked) == 0 {
		return nil, nil, false, fmt.Errorf("no words available — every level appears exhausted")
	}

	ids := make([]int64, len(picked))
	wordPtrs := make([]*srs.Word, len(picked))
	for i, w := range picked {
		w := w
		ids[i] = w.ID
		wordPtrs[i] = &w
	}
	created, err := s.repo.CreateSession(ctx, userID, s.now(), ids)
	if err != nil {
		return nil, nil, false, fmt.Errorf("create session: %w", err)
	}
	return created, wordPtrs, false, nil
}

// Status assembles the opening-screen counters for the given session.
func (s *Service) Status(ctx context.Context, userID int64, session *Session) (StatusView, error) {
	progress, err := s.repo.BulkGetProgress(ctx, userID, session.WordIDs)
	if err != nil {
		return StatusView{}, err
	}
	view := StatusView{
		WordsInSession: len(session.WordIDs),
		ModesTotal:     len(session.WordIDs) * len(AllModes) * ThresholdPerMode,
	}
	for _, wid := range session.WordIDs {
		p := progress[wid]
		if p.Learned {
			view.LearnedThisRound++
		}
		for _, m := range AllModes {
			c := p.Counts[m]
			if c > ThresholdPerMode {
				c = ThresholdPerMode
			}
			view.ModesCompleted += c
		}
	}
	if total, err := s.repo.CountLearnedViaWords(ctx, userID); err == nil {
		view.LearnedAllTime = total
	}
	if levels, err := s.LevelProgress(ctx, userID); err == nil {
		view.ByLevel = levels
	}
	return view, nil
}

// LevelProgress returns per-CEFR-level (learned, total) tuples for the user.
// Levels with Total = 0 (no words seeded yet for that level) are dropped, so
// callers can render the result as-is. The result is recomputed live on every
// call — when new words are added to data/words.json and seeded into the words
// table at startup, the denominator updates automatically.
func (s *Service) LevelProgress(ctx context.Context, userID int64) ([]LevelProgress, error) {
	rows, err := s.repo.CountWordsByLevel(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]LevelProgress, 0, len(rows))
	for _, lp := range rows {
		if lp.Total == 0 {
			continue
		}
		out = append(out, lp)
	}
	return out, nil
}

// NextSlot returns the next slot to display for an open session, plus the
// session itself (refreshed) and a sessionDone flag.
func (s *Service) NextSlot(ctx context.Context, sessionID int64) (*Slot, *Session, bool, error) {
	session, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, nil, false, err
	}
	if session == nil {
		return nil, nil, false, ErrSessionNotFound
	}
	if session.CompletedAt != nil {
		return nil, session, true, nil
	}

	wordsByID, err := s.loadSessionWordsMap(ctx, session.WordIDs)
	if err != nil {
		return nil, session, false, err
	}
	progress, err := s.repo.BulkGetProgress(ctx, session.UserID, session.WordIDs)
	if err != nil {
		return nil, session, false, err
	}

	slot, done := NextSlot(session, wordsByID, progress, s.now())
	if done {
		_ = s.repo.CompleteSession(ctx, session.ID)
		session.CompletedAt = ptrTime(s.now())
		return nil, session, true, nil
	}
	return &slot, session, false, nil
}

// SubmitMC records an MC answer (handler-side correctness). Returns the
// outcome plus whether the word just became learned.
func (s *Service) SubmitMC(ctx context.Context, session *Session, wordID int64, mode Mode, correct bool) (Outcome, error) {
	if !slotInSession(session, wordID) {
		return Outcome{}, ErrWordNotInSession
	}
	word, err := s.srsRepo.GetWordByID(ctx, wordID)
	if err != nil {
		return Outcome{}, fmt.Errorf("get word: %w", err)
	}
	out, err := s.recordAnswer(ctx, session, wordID, mode, correct)
	if err != nil {
		return Outcome{}, err
	}
	out.Expected = expectedAnswer(word, mode)
	return out, nil
}

// SubmitTyping validates a typed answer using the textmatch package and
// records the result.
func (s *Service) SubmitTyping(ctx context.Context, session *Session, wordID int64, mode Mode, raw string) (Outcome, error) {
	if !slotInSession(session, wordID) {
		return Outcome{}, ErrWordNotInSession
	}
	word, err := s.srsRepo.GetWordByID(ctx, wordID)
	if err != nil {
		return Outcome{}, fmt.Errorf("get word: %w", err)
	}
	expected := expectedAnswer(word, mode)
	correct := textmatch.Match(raw, expected)
	out, err := s.recordAnswer(ctx, session, wordID, mode, correct)
	if err != nil {
		return Outcome{}, err
	}
	out.Expected = expected
	return out, nil
}

// SubmitSkip records an "I don't know" press as a wrong answer (resets the
// per-mode counter to 0).
func (s *Service) SubmitSkip(ctx context.Context, session *Session, wordID int64, mode Mode) (Outcome, error) {
	if !slotInSession(session, wordID) {
		return Outcome{}, ErrWordNotInSession
	}
	word, err := s.srsRepo.GetWordByID(ctx, wordID)
	if err != nil {
		return Outcome{}, fmt.Errorf("get word: %w", err)
	}
	out, err := s.recordAnswer(ctx, session, wordID, mode, false)
	if err != nil {
		return Outcome{}, err
	}
	out.Expected = expectedAnswer(word, mode)
	return out, nil
}

// MarkAlreadyKnown stamps all 4 mode counters to 2, marks learned, and
// ensures a SM-2 review_card so the word enters the spaced-repetition cycle.
func (s *Service) MarkAlreadyKnown(ctx context.Context, session *Session, wordID int64) error {
	if !slotInSession(session, wordID) {
		return ErrWordNotInSession
	}
	if err := s.repo.MarkAlreadyKnown(ctx, session.UserID, wordID); err != nil {
		return fmt.Errorf("mark already known: %w", err)
	}
	return s.ensureReviewCard(ctx, session.UserID, wordID)
}

// CompleteSession ends a session early.
func (s *Service) CompleteSession(ctx context.Context, sessionID int64) error {
	return s.repo.CompleteSession(ctx, sessionID)
}

// --- Repository proxies for handler convenience ---

// RepoOpenSession returns the user's currently-open session, or nil if none.
func (s *Service) RepoOpenSession(ctx context.Context, userID int64) (*Session, error) {
	return s.repo.GetOpenSession(ctx, userID)
}

// RepoGetSessionByID returns a session by ID (or nil if not found).
func (s *Service) RepoGetSessionByID(ctx context.Context, sessionID int64) (*Session, error) {
	return s.repo.GetSessionByID(ctx, sessionID)
}

// RepoGetProgress returns the (user, word) progress row, or nil if no row exists.
func (s *Service) RepoGetProgress(ctx context.Context, userID, wordID int64) (*Progress, error) {
	return s.repo.GetProgress(ctx, userID, wordID)
}

// CountInProgress returns how many words the user has started learning but
// not yet finished (any of 4 mode counters < 2).
func (s *Service) CountInProgress(ctx context.Context, userID int64) (int, error) {
	return s.repo.CountInProgress(ctx, userID)
}

// CountLearnedViaWords returns how many words the user has fully drilled
// through /words (all 4 modes ≥ 2). This is distinct from
// srs.Repository.CountLearnedWords which counts SM-2 stability via repetitions.
func (s *Service) CountLearnedViaWords(ctx context.Context, userID int64) (int, error) {
	return s.repo.CountLearnedViaWords(ctx, userID)
}

// --- internal helpers ---

func (s *Service) recordAnswer(ctx context.Context, session *Session, wordID int64, mode Mode, correct bool) (Outcome, error) {
	if correct {
		if err := s.repo.IncProgress(ctx, session.UserID, wordID, mode); err != nil {
			return Outcome{}, fmt.Errorf("inc progress: %w", err)
		}
	} else {
		if err := s.repo.ResetProgress(ctx, session.UserID, wordID, mode); err != nil {
			return Outcome{}, fmt.Errorf("reset progress: %w", err)
		}
		// Track session-level errors only on wrong/skip; correct doesn't grow them.
		_ = s.repo.IncSessionWordError(ctx, session.ID, wordID)
	}

	// Update anti-repeat cursor.
	_ = s.repo.UpdateLastWord(ctx, session.ID, wordID)

	out := Outcome{Correct: correct}
	if errs, err := s.repo.GetSessionWordError(ctx, session.ID, wordID); err == nil {
		out.ErrorsInSession = errs
	}

	// Check learn-promotion only on correct answers.
	if correct {
		p, err := s.repo.GetProgress(ctx, session.UserID, wordID)
		if err != nil {
			return out, fmt.Errorf("get progress after inc: %w", err)
		}
		if p != nil && !p.Learned && p.AllModesPassed() {
			if err := s.repo.MarkLearned(ctx, session.UserID, wordID); err != nil {
				return out, fmt.Errorf("mark learned: %w", err)
			}
			if err := s.ensureReviewCard(ctx, session.UserID, wordID); err != nil {
				return out, fmt.Errorf("ensure review card: %w", err)
			}
			out.BecameLearned = true
		}
	}
	return out, nil
}

// DeferWord removes wordID from the active session — the slot picker will skip
// it for the rest of this session. Progress in word_learning_progress is
// untouched, so on a future session the word reappears as in-progress.
func (s *Service) DeferWord(ctx context.Context, session *Session, wordID int64) error {
	if !slotInSession(session, wordID) {
		return ErrWordNotInSession
	}
	if err := s.repo.DeferSessionWord(ctx, session.ID, wordID); err != nil {
		return fmt.Errorf("defer word: %w", err)
	}
	return nil
}

// ensureReviewCard creates a SM-2 review_card for (user, word) if one doesn't
// exist yet. Schedules first review for tomorrow with default ease factor.
// Idempotent: if a card already exists, leaves it untouched.
func (s *Service) ensureReviewCard(ctx context.Context, userID, wordID int64) error {
	existing, err := s.srsRepo.GetCardByUserWord(ctx, userID, wordID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	card := &srs.ReviewCard{
		UserID:         userID,
		WordID:         wordID,
		EaseFactor:     2.5,
		IntervalDays:   1,
		Repetitions:    0,
		NextReviewDate: s.now().AddDate(0, 0, 1),
	}
	return s.srsRepo.UpsertCard(ctx, card)
}

func (s *Service) loadSessionWords(ctx context.Context, ids []int64) ([]*srs.Word, error) {
	out := make([]*srs.Word, 0, len(ids))
	for _, id := range ids {
		w, err := s.srsRepo.GetWordByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get word %d: %w", id, err)
		}
		out = append(out, w)
	}
	return out, nil
}

func (s *Service) loadSessionWordsMap(ctx context.Context, ids []int64) (map[int64]*srs.Word, error) {
	out := make(map[int64]*srs.Word, len(ids))
	for _, id := range ids {
		w, err := s.srsRepo.GetWordByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get word %d: %w", id, err)
		}
		out[id] = w
	}
	return out, nil
}

func slotInSession(session *Session, wordID int64) bool {
	for _, id := range session.WordIDs {
		if id == wordID {
			return true
		}
	}
	return false
}

// expectedAnswer returns the "correct answer" string for the given mode.
// For typing modes the result may contain ";"-separated alternatives, which
// textmatch.Match handles.
func expectedAnswer(word *srs.Word, mode Mode) string {
	switch mode {
	case ModeEnRuMC, ModeEnRuTyping:
		return word.Russian
	case ModeRuEnMC, ModeRuEnTyping:
		return word.English
	default:
		return ""
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
