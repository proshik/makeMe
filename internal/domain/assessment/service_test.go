package assessment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

// mockRepo is an in-memory implementation of Repository used in unit tests.
type mockRepo struct {
	sessions map[int64]*Session
	answers  []*Answer
	nextID   int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{sessions: make(map[int64]*Session)}
}

func (r *mockRepo) CreateSession(_ context.Context, userID int64) (*Session, error) {
	r.nextID++
	s := &Session{ID: r.nextID, UserID: userID, Status: StatusInProgress, CurrentLevel: user.B1}
	r.sessions[r.nextID] = s
	return s, nil
}

func (r *mockRepo) GetActiveSession(_ context.Context, userID int64) (*Session, error) {
	for _, s := range r.sessions {
		if s.UserID == userID && s.Status == StatusInProgress {
			cp := *s
			return &cp, nil
		}
	}
	return nil, ErrNoActiveSession
}

func (r *mockRepo) UpdateSession(_ context.Context, s *Session) error {
	existing := r.sessions[s.ID]
	if existing == nil {
		return ErrNoActiveSession
	}
	*existing = *s
	return nil
}

func (r *mockRepo) AddAnswer(_ context.Context, a *Answer) error {
	r.answers = append(r.answers, a)
	return nil
}

func (r *mockRepo) CountAnswers(_ context.Context, sessionID int64) (int, error) {
	count := 0
	for _, a := range r.answers {
		if a.SessionID == sessionID {
			count++
		}
	}
	return count, nil
}

func (r *mockRepo) CountCorrectByLevel(_ context.Context, sessionID int64) (map[string]LevelStats, error) {
	stats := make(map[string]LevelStats)
	for _, a := range r.answers {
		if a.SessionID == sessionID && !a.Skipped {
			s := stats[string(a.QuestionLevel)]
			s.Total++
			if a.Correct {
				s.Correct++
			}
			stats[string(a.QuestionLevel)] = s
		}
	}
	return stats, nil
}

func (r *mockRepo) GetAnsweredTexts(_ context.Context, sessionID int64) ([]string, error) {
	var texts []string
	for _, a := range r.answers {
		if a.SessionID == sessionID {
			texts = append(texts, a.QuestionText)
		}
	}
	return texts, nil
}

func (r *mockRepo) GetSessionAnswers(_ context.Context, sessionID int64) ([]*Answer, error) {
	var answers []*Answer
	for _, a := range r.answers {
		if a.SessionID == sessionID {
			answers = append(answers, a)
		}
	}
	return answers, nil
}

func (r *mockRepo) GetLastCompleted(_ context.Context, userID int64) (*Session, error) {
	return nil, ErrNoActiveSession
}

// TestSubmitAnswer_UsesShownQuestion verifies that SubmitAnswer records the answer
// against the question that was actually shown to the user, not a randomly picked new one.
func TestSubmitAnswer_UsesShownQuestion(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := NewService(repo, nil, nil, nil)

	// Start session — picks and returns the first question Q1.
	session, shownQ, err := svc.Start(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, shownQ)

	// After Start, the session's pending question must be stored so SubmitAnswer can use it.
	activeSession, err := repo.GetActiveSession(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, activeSession.PendingQuestion, "Start must store the shown question as PendingQuestion in session")
	assert.Equal(t, shownQ.Text, activeSession.PendingQuestion.Text)

	// Submit Q1's correct option.
	_, nextQ, answers, err := svc.SubmitAnswer(ctx, 1, shownQ.CorrectOption)
	require.NoError(t, err)
	require.Nil(t, answers)  // session not complete yet
	require.NotNil(t, nextQ) // next question is ready

	// The stored answer must reference the SHOWN question, not any random new one.
	storedAnswers, err := repo.GetSessionAnswers(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, storedAnswers, 1)
	assert.Equal(t, shownQ.Text, storedAnswers[0].QuestionText, "stored QuestionText must match the shown question")
	assert.True(t, storedAnswers[0].Correct, "submitting the correct option must be recorded as correct")
	assert.Equal(t, shownQ.CorrectOption, storedAnswers[0].CorrectOption)
}

// TestSubmitAnswer_WrongAnswerRecordedCorrectly verifies that submitting a wrong option
// is recorded as incorrect with correct book-keeping.
func TestSubmitAnswer_WrongAnswerRecordedCorrectly(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := NewService(repo, nil, nil, nil)

	session, shownQ, err := svc.Start(ctx, 2)
	require.NoError(t, err)

	wrongOption := (shownQ.CorrectOption + 1) % len(shownQ.Options)
	_, _, _, err = svc.SubmitAnswer(ctx, 2, wrongOption)
	require.NoError(t, err)

	storedAnswers, err := repo.GetSessionAnswers(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, storedAnswers, 1)
	assert.False(t, storedAnswers[0].Correct)
	assert.Equal(t, shownQ.CorrectOption, storedAnswers[0].CorrectOption)
	assert.Equal(t, wrongOption, storedAnswers[0].UserOption)
}

// TestSubmitAnswer_SkipIsNotAnError verifies that "I don't know" (SkipOption) is recorded
// as skipped+incorrect but does not tally as an error in level stats.
func TestSubmitAnswer_SkipIsNotAnError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	svc := NewService(repo, nil, nil, nil)

	session, _, err := svc.Start(ctx, 3)
	require.NoError(t, err)

	_, _, _, err = svc.SubmitAnswer(ctx, 3, SkipOption)
	require.NoError(t, err)

	storedAnswers, err := repo.GetSessionAnswers(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, storedAnswers, 1)
	assert.True(t, storedAnswers[0].Skipped)
	assert.False(t, storedAnswers[0].Correct)
}

func TestAdjustLevel(t *testing.T) {
	tests := []struct {
		name        string
		current     user.Level
		correct     bool
		consecutive int
		wantLevel   user.Level
		wantConsec  int
	}{
		{"first correct from B1 stays (consecutive=0)", user.B1, true, 0, user.B1, 1},
		{"second correct from B1 goes up (consecutive=1)", user.B1, true, 1, user.B2, 0},
		{"wrong from B1 goes down", user.B1, false, 1, user.A2, 0},
		{"second correct from C2 stays at C2", user.C2, true, 1, user.C2, 0},
		{"wrong from A1 stays", user.A1, false, 0, user.A1, 0},
		{"second correct from A1 goes up", user.A1, true, 1, user.A2, 0},
		{"wrong from C2 goes down", user.C2, false, 2, user.C1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, gotConsec := adjustLevel(tt.current, tt.correct, tt.consecutive)
			if gotLevel != tt.wantLevel {
				t.Errorf("adjustLevel(%s, %v, %d) level = %s, want %s", tt.current, tt.correct, tt.consecutive, gotLevel, tt.wantLevel)
			}
			if gotConsec != tt.wantConsec {
				t.Errorf("adjustLevel(%s, %v, %d) consecutive = %d, want %d", tt.current, tt.correct, tt.consecutive, gotConsec, tt.wantConsec)
			}
		})
	}
}

func TestDetermineLevel(t *testing.T) {
	tests := []struct {
		name  string
		stats map[string]LevelStats
		want  user.Level
	}{
		{
			name:  "all correct at B1",
			stats: map[string]LevelStats{"B1": {Correct: 5, Total: 5}},
			want:  user.B1,
		},
		{
			name:  "correct at multiple levels picks highest",
			stats: map[string]LevelStats{"A1": {3, 3}, "A2": {2, 3}, "B1": {1, 2}},
			want:  user.B1,
		},
		{
			name:  "below 50% doesn't count",
			stats: map[string]LevelStats{"A1": {3, 3}, "A2": {1, 3}},
			want:  user.A1,
		},
		{
			name:  "empty stats defaults to A1",
			stats: map[string]LevelStats{},
			want:  user.A1,
		},
		{
			name:  "all wrong defaults to A1",
			stats: map[string]LevelStats{"B1": {0, 5}},
			want:  user.A1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineLevel(tt.stats)
			if got != tt.want {
				t.Errorf("determineLevel() = %s, want %s", got, tt.want)
			}
		})
	}
}

// levelIndex must not inherit user.Level.Index()'s -1 for unknown values: the
// adaptive arithmetic indexes the ladder directly, and -1 would panic. A
// corrupted session should keep asking answerable questions instead.
func TestLevelIndexFallsBackToMiddleForUnknown(t *testing.T) {
	for _, l := range []user.Level{"", "Z9", "b1"} {
		if got := levelIndex(l); got != 2 {
			t.Errorf("levelIndex(%q) = %d, want 2 (B1)", l, got)
		}
	}
	// Known levels still map to their real position.
	for i, l := range user.AllLevels() {
		if got := levelIndex(l); got != i {
			t.Errorf("levelIndex(%q) = %d, want %d", l, got, i)
		}
	}
}

// adjustLevel does index arithmetic on the ladder; the ends must clamp rather
// than run off it.
func TestAdjustLevelClampsAtTheEnds(t *testing.T) {
	if got, _ := adjustLevel(user.A1, false, 0); got != user.A1 {
		t.Errorf("a wrong answer at A1 = %q, want A1 (nothing below)", got)
	}
	if got, _ := adjustLevel(user.C2, true, 1); got != user.C2 {
		t.Errorf("a second correct answer at C2 = %q, want C2 (nothing above)", got)
	}
	if got, _ := adjustLevel(user.B1, true, 1); got != user.B2 {
		t.Errorf("two correct answers at B1 = %q, want B2", got)
	}
	if got, _ := adjustLevel(user.B1, false, 0); got != user.A2 {
		t.Errorf("a wrong answer at B1 = %q, want A2", got)
	}
}
