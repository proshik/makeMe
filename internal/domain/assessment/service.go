package assessment

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
)

const (
	totalQuestions    = 30
	vocabPhaseCount   = 15 // questions 1–15: vocabulary
	grammarPhaseCount = 15 // questions 16–30: grammar
)

// SkipOption is the sentinel value for userOption meaning "I don't know".
const SkipOption = -1

var (
	ErrNoActiveSession = errors.New("no active assessment session")
	ErrSessionComplete = errors.New("assessment session already completed")
)

// levels ordered by difficulty for navigation.
var levels = []user.Level{user.A1, user.A2, user.B1, user.B2, user.C1, user.C2}

type Service struct {
	repo        Repository
	userSvc     *user.Service
	srsRepo     srs.Repository
	grammarRepo task.GrammarRepository
}

func NewService(repo Repository, userSvc *user.Service, srsRepo srs.Repository, grammarRepo task.GrammarRepository) *Service {
	return &Service{repo: repo, userSvc: userSvc, srsRepo: srsRepo, grammarRepo: grammarRepo}
}

// Start creates a new assessment session starting at B1, or resumes an existing one.
// The picked question is stored as PendingQuestion so SubmitAnswer can verify the answer
// against the exact question that was shown to the user.
func (s *Service) Start(ctx context.Context, userID int64) (*Session, *Question, error) {
	existing, err := s.repo.GetActiveSession(ctx, userID)
	if err != nil && !errors.Is(err, ErrNoActiveSession) {
		return nil, nil, fmt.Errorf("check active session: %w", err)
	}
	if existing != nil {
		answered, _ := s.repo.CountAnswers(ctx, existing.ID)
		exclude, _ := s.buildExcludeSet(ctx, existing.ID)
		q, err := s.pickQuestion(ctx, existing.CurrentLevel, exclude, answered)
		if err != nil {
			return nil, nil, err
		}
		existing.PendingQuestion = q
		if err := s.repo.UpdateSession(ctx, existing); err != nil {
			return nil, nil, fmt.Errorf("store pending question: %w", err)
		}
		return existing, q, nil
	}

	session, err := s.repo.CreateSession(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("create session: %w", err)
	}

	q, err := s.pickQuestion(ctx, user.A1, nil, 0)
	if err != nil {
		return nil, nil, err
	}
	session.PendingQuestion = q
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("store pending question: %w", err)
	}
	return session, q, nil
}

// SubmitAnswer records the user's answer, adjusts difficulty, and returns
// the next question (nil if done) and the full answer list (non-nil only when done).
func (s *Service) SubmitAnswer(ctx context.Context, userID int64, userOption int) (*Session, *Question, []*Answer, error) {
	session, err := s.repo.GetActiveSession(ctx, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get active session: %w", err)
	}
	if session.Status == StatusCompleted {
		return session, nil, nil, ErrSessionComplete
	}

	// Use the question that was stored when the question was shown to the user.
	q := session.PendingQuestion
	if q == nil {
		return nil, nil, nil, fmt.Errorf("no pending question for session %d", session.ID)
	}

	skipped := userOption == SkipOption
	correct := !skipped && userOption == q.CorrectOption
	answer := &Answer{
		SessionID:     session.ID,
		QuestionLevel: session.CurrentLevel,
		QuestionText:  q.Text,
		Options:       q.Options,
		CorrectOption: q.CorrectOption,
		UserOption:    userOption,
		Correct:       correct,
		Skipped:       skipped,
	}
	if err := s.repo.AddAnswer(ctx, answer); err != nil {
		return nil, nil, nil, fmt.Errorf("add answer: %w", err)
	}

	count, err := s.repo.CountAnswers(ctx, session.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("count answers: %w", err)
	}

	if count >= totalQuestions {
		session, answers, err := s.completeSession(ctx, session)
		return session, nil, answers, err
	}

	// Treat skipped as incorrect for adaptive level adjustment.
	nextLevel, nextConsecutive := adjustLevel(session.CurrentLevel, correct, session.ConsecutiveCorrect)

	// Reset level to A1 at the grammar phase boundary.
	if int(count) == vocabPhaseCount {
		nextLevel = user.A1
		nextConsecutive = 0
	}

	exclude, _ := s.buildExcludeSet(ctx, session.ID)
	nextQ, err := s.pickQuestion(ctx, nextLevel, exclude, int(count))
	if err != nil {
		return nil, nil, nil, err
	}

	session.CurrentLevel = nextLevel
	session.ConsecutiveCorrect = nextConsecutive
	session.PendingQuestion = nextQ
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, nil, nil, fmt.Errorf("update session: %w", err)
	}

	return session, nextQ, nil, nil
}

// TotalQuestions is how many questions one assessment run has. Exposed so
// adapters can render "question N of M" without duplicating the constant.
func (s *Service) TotalQuestions() int { return totalQuestions }

// VocabPhaseCount is where the run switches from vocabulary to grammar.
func (s *Service) VocabPhaseCount() int { return vocabPhaseCount }

// GetActiveSession returns the current in-progress session or ErrNoActiveSession.
func (s *Service) GetActiveSession(ctx context.Context, userID int64) (*Session, error) {
	return s.repo.GetActiveSession(ctx, userID)
}

// AnswerCount returns how many answers have been recorded for the session.
func (s *Service) AnswerCount(ctx context.Context, sessionID int64) (int, error) {
	return s.repo.CountAnswers(ctx, sessionID)
}

func (s *Service) completeSession(ctx context.Context, session *Session) (*Session, []*Answer, error) {
	stats, err := s.repo.CountCorrectByLevel(ctx, session.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("count correct by level: %w", err)
	}

	determined := determineLevel(stats)
	session.Status = StatusCompleted
	session.DeterminedLevel = determined

	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("complete session: %w", err)
	}

	if err := s.userSvc.UpdateLevel(ctx, session.UserID, determined); err != nil {
		return nil, nil, fmt.Errorf("update user level: %w", err)
	}

	answers, err := s.repo.GetSessionAnswers(ctx, session.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("get session answers: %w", err)
	}

	return session, answers, nil
}

// buildExcludeSet returns a set of question texts already answered in the session.
func (s *Service) buildExcludeSet(ctx context.Context, sessionID int64) (map[string]bool, error) {
	texts, err := s.repo.GetAnsweredTexts(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(texts))
	for _, t := range texts {
		set[t] = true
	}
	return set, nil
}

// pickQuestion selects a question based on the current phase.
// Phase 1 (answered < vocabPhaseCount): vocabulary only.
// Phase 2 (answered >= vocabPhaseCount): grammar only.
func (s *Service) pickQuestion(ctx context.Context, level user.Level, exclude map[string]bool, answeredSoFar int) (*Question, error) {
	if answeredSoFar < vocabPhaseCount {
		if s.srsRepo != nil {
			q, err := s.generateVocabQuestion(ctx, level, exclude)
			if err == nil && q != nil {
				return q, nil
			}
		}
		// Fall back to grammar if no vocab available
	}
	return s.pickGrammarQuestion(ctx, level, exclude), nil
}

func (s *Service) pickGrammarQuestion(ctx context.Context, level user.Level, exclude map[string]bool) *Question {
	if s.grammarRepo != nil {
		if q := s.pickGrammarFromDB(ctx, level, exclude); q != nil {
			return q
		}
	}
	// Fallback to hardcoded bank (used in tests or if DB is empty).
	questions := questionBank[level]
	if len(questions) == 0 {
		questions = questionBank[user.B1]
	}
	available := filterExcluded(questions, exclude)
	if len(available) == 0 {
		available = questions
	}
	q := available[rand.IntN(len(available))]
	return &q
}

func (s *Service) pickGrammarFromDB(ctx context.Context, level user.Level, exclude map[string]bool) *Question {
	rows, err := s.grammarRepo.GetByLevel(ctx, level)
	if err != nil || len(rows) == 0 {
		// Try fallback level B1.
		if level != user.B1 {
			rows, err = s.grammarRepo.GetByLevel(ctx, user.B1)
		}
		if err != nil || len(rows) == 0 {
			return nil
		}
	}

	questions := make([]Question, 0, len(rows))
	for _, gq := range rows {
		questions = append(questions, dbGrammarToQuestion(gq))
	}

	available := filterExcluded(questions, exclude)
	if len(available) == 0 {
		available = questions
	}
	q := available[rand.IntN(len(available))]
	return &q
}

// dbGrammarToQuestion converts a DB grammar row to an assessment Question.
// The Answer field holds the correct option text; we find its index in Options.
func dbGrammarToQuestion(gq task.GrammarQuestion) Question {
	correctIdx := 0
	for i, opt := range gq.Options {
		if opt == gq.Answer {
			correctIdx = i
			break
		}
	}
	return Question{
		Level:         gq.Level,
		Text:          gq.Prompt,
		Options:       gq.Options,
		CorrectOption: correctIdx,
	}
}

// generateVocabQuestion builds a "translate the word" question from the SRS words table.
func (s *Service) generateVocabQuestion(ctx context.Context, level user.Level, exclude map[string]bool) (*Question, error) {
	words, err := s.srsRepo.GetWordsByLevel(ctx, level, 20)
	if err != nil || len(words) == 0 {
		return nil, fmt.Errorf("no words for level %s", level)
	}

	// Pick a word whose question text is not already in exclude
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })

	var targetIdx int = -1
	for i, w := range words {
		text := "Как переводится: «" + w.English + "»?"
		if !exclude[text] {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		return nil, fmt.Errorf("all vocab questions at level %s already asked", level)
	}

	target := words[targetIdx]
	questionText := "Как переводится: «" + target.English + "»?"

	// Build distractors: 3 other Russian translations from the same level
	distractors := make([]string, 0, 3)
	for i, w := range words {
		if i == targetIdx {
			continue
		}
		if w.Russian != target.Russian {
			distractors = append(distractors, w.Russian)
		}
		if len(distractors) == 3 {
			break
		}
	}

	// If not enough distractors from this level, pad from adjacent level
	if len(distractors) < 3 {
		adjacent := adjacentLevel(level)
		extras, _ := s.srsRepo.GetWordsByLevel(ctx, adjacent, 10)
		for _, w := range extras {
			if w.Russian != target.Russian {
				distractors = append(distractors, w.Russian)
			}
			if len(distractors) == 3 {
				break
			}
		}
	}

	if len(distractors) < 3 {
		return nil, fmt.Errorf("not enough distractors for vocab question")
	}

	// Shuffle options and record correct index
	options := append([]string{target.Russian}, distractors[:3]...)
	rand.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })

	correctIdx := 0
	for i, o := range options {
		if o == target.Russian {
			correctIdx = i
			break
		}
	}

	return &Question{
		Level:         level,
		Text:          questionText,
		Options:       options,
		CorrectOption: correctIdx,
	}, nil
}

func adjacentLevel(l user.Level) user.Level {
	idx := levelIndex(l)
	if idx > 0 {
		return levels[idx-1]
	}
	return levels[1] // A2
}

func filterExcluded(questions []Question, exclude map[string]bool) []Question {
	if len(exclude) == 0 {
		return questions
	}
	out := make([]Question, 0, len(questions))
	for _, q := range questions {
		if !exclude[q.Text] {
			out = append(out, q)
		}
	}
	return out
}

// adjustLevel moves one step up after 2 consecutive correct answers, one step down on incorrect.
// Returns the new level and new consecutive-correct counter.
func adjustLevel(current user.Level, correct bool, consecutive int) (user.Level, int) {
	if !correct {
		idx := levelIndex(current)
		if idx > 0 {
			return levels[idx-1], 0
		}
		return current, 0
	}
	// Correct answer
	newConsecutive := consecutive + 1
	if newConsecutive >= 2 {
		idx := levelIndex(current)
		if idx < len(levels)-1 {
			return levels[idx+1], 0
		}
		return current, 0
	}
	return current, newConsecutive
}

func levelIndex(l user.Level) int {
	for i, lv := range levels {
		if lv == l {
			return i
		}
	}
	return 2 // default B1
}

// determineLevel picks the highest consecutive level (starting from A1) where
// the user got >= 50% correct. Stops at the first failing level that was tested.
// Levels with no questions are skipped without breaking the chain.
func determineLevel(stats map[string]LevelStats) user.Level {
	best := user.A1
	for _, lv := range levels {
		s, ok := stats[string(lv)]
		if !ok {
			// No questions asked at this level — skip without penalising.
			continue
		}
		if s.Total == 0 {
			continue
		}
		if float64(s.Correct)/float64(s.Total) < 0.5 {
			// Failed this level — stop here.
			break
		}
		best = lv
	}
	return best
}
