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
	require.Nil(t, answers)   // session not complete yet
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
		name    string
		current user.Level
		correct bool
		want    user.Level
	}{
		{"correct from B1 goes up", user.B1, true, user.B2},
		{"wrong from B1 goes down", user.B1, false, user.A2},
		{"correct from C2 stays", user.C2, true, user.C2},
		{"wrong from A1 stays", user.A1, false, user.A1},
		{"correct from A1 goes up", user.A1, true, user.A2},
		{"wrong from C2 goes down", user.C2, false, user.C1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustLevel(tt.current, tt.correct)
			if got != tt.want {
				t.Errorf("adjustLevel(%s, %v) = %s, want %s", tt.current, tt.correct, got, tt.want)
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
