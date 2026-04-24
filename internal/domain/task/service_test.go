package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

// fakeRepo is an in-memory Repository for service tests.
type fakeRepo struct {
	nextSetID  int64
	nextTaskID int64
	sets       []DailySet
	tasks      map[int64][]Task // keyed by dailySetID
	results    []TaskResult
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		tasks: make(map[int64][]Task),
	}
}

func (r *fakeRepo) CreateDailySet(ctx context.Context, userID, groupID int64, date time.Time, round int) (*DailySet, error) {
	r.nextSetID++
	ds := DailySet{
		ID:        r.nextSetID,
		UserID:    userID,
		GroupID:   groupID,
		Date:      date,
		CreatedAt: time.Now(),
		Round:     round,
	}
	r.sets = append(r.sets, ds)
	return &ds, nil
}

func (r *fakeRepo) GetDailySet(ctx context.Context, userID, groupID int64, date time.Time) (*DailySet, error) {
	var latest *DailySet
	for i := range r.sets {
		s := &r.sets[i]
		if s.UserID == userID && s.GroupID == groupID && s.Date.Equal(date) {
			if latest == nil || s.Round > latest.Round {
				latest = s
			}
		}
	}
	if latest == nil {
		return nil, ErrNoDailySet
	}
	return latest, nil
}

func (r *fakeRepo) AddTask(ctx context.Context, t *Task) error {
	r.nextTaskID++
	t.ID = r.nextTaskID
	r.tasks[t.DailySetID] = append(r.tasks[t.DailySetID], *t)
	return nil
}

func (r *fakeRepo) GetTasksByDailySet(ctx context.Context, dailySetID int64) ([]Task, error) {
	return r.tasks[dailySetID], nil
}

func (r *fakeRepo) GetTaskByID(ctx context.Context, taskID int64) (*Task, error) {
	for _, ts := range r.tasks {
		for i := range ts {
			if ts[i].ID == taskID {
				return &ts[i], nil
			}
		}
	}
	return nil, errors.New("not found")
}

func (r *fakeRepo) AddResult(ctx context.Context, res *TaskResult) error {
	r.results = append(r.results, *res)
	return nil
}

func (r *fakeRepo) GetResult(ctx context.Context, taskID, userID int64) (*TaskResult, error) {
	for i := range r.results {
		if r.results[i].TaskID == taskID && r.results[i].UserID == userID {
			return &r.results[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (r *fakeRepo) CountResults(ctx context.Context, dailySetID, userID int64) (int, int, error) {
	total := len(r.tasks[dailySetID])
	answered := 0
	for _, t := range r.tasks[dailySetID] {
		for _, res := range r.results {
			if res.TaskID == t.ID && res.UserID == userID {
				answered++
				break
			}
		}
	}
	return answered, total, nil
}

func (r *fakeRepo) CountCorrectResults(ctx context.Context, dailySetID, userID int64) (int, error) {
	correct := 0
	for _, t := range r.tasks[dailySetID] {
		for _, res := range r.results {
			if res.TaskID == t.ID && res.UserID == userID && res.Correct {
				correct++
				break
			}
		}
	}
	return correct, nil
}

// answerAll marks every task in the set as correctly answered by the user.
func (r *fakeRepo) answerAll(dailySetID, userID int64) {
	for _, t := range r.tasks[dailySetID] {
		r.results = append(r.results, TaskResult{
			TaskID:  t.ID,
			UserID:  userID,
			Correct: true,
			Score:   pointsPerTask,
		})
	}
}

// stubGenerator produces a fixed number of placeholder tasks.
type stubGenerator struct {
	taskType TaskType
}

func (g *stubGenerator) Type() TaskType { return g.taskType }

func (g *stubGenerator) Generate(ctx context.Context, userID int64, level user.Level, count int) ([]Task, error) {
	out := make([]Task, count)
	for i := range out {
		out[i] = Task{
			Type:   g.taskType,
			Level:  level,
			Prompt: "stub",
			Answer: "a",
		}
	}
	return out, nil
}

// noopSRSRepo satisfies srs.Repository for service tests that don't hit SRS.
type noopSRSRepo struct{}

func (noopSRSRepo) GetDueCards(ctx context.Context, userID int64, limit int) ([]srs.ReviewCard, error) {
	return nil, nil
}
func (noopSRSRepo) UpsertCard(ctx context.Context, card *srs.ReviewCard) error { return nil }
func (noopSRSRepo) GetWordsByLevel(ctx context.Context, level user.Level, limit int) ([]srs.Word, error) {
	return nil, nil
}
func (noopSRSRepo) GetWordByID(ctx context.Context, id int64) (*srs.Word, error) { return nil, nil }
func (noopSRSRepo) GetNewWordsForUser(ctx context.Context, userID int64, level user.Level, limit int) ([]srs.Word, error) {
	return nil, nil
}
func (noopSRSRepo) UpdateWordAudio(ctx context.Context, wordID int64, transcription, audioURL string) error {
	return nil
}
func (noopSRSRepo) CountLearnedWords(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

// ──────────────────────────────────────────────────────────────────────────────

func newServiceWithMax(repo Repository, maxRounds int) *Service {
	vocab := &stubGenerator{taskType: TypeVocab}
	grammar := &stubGenerator{taskType: TypeGrammar}
	return NewService(repo, noopSRSRepo{}, maxRounds, vocab, grammar)
}

func TestGenerateDaily_ReturnsErrDailyLimitReached_WhenMaxRoundsCompleted(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithMax(repo, 2)
	ctx := context.Background()

	// Round 0
	ds0, err := svc.GenerateDaily(ctx, 1, 10, user.A1)
	require.NoError(t, err)
	require.Equal(t, 0, ds0.Round)
	repo.answerAll(ds0.ID, 1)

	// Round 1
	ds1, err := svc.GenerateDaily(ctx, 1, 10, user.A1)
	require.NoError(t, err)
	require.Equal(t, 1, ds1.Round)
	repo.answerAll(ds1.ID, 1)

	// maxRounds=2 → after completing rounds 0 and 1, next /tasks should be blocked
	_, err = svc.GenerateDaily(ctx, 1, 10, user.A1)
	assert.ErrorIs(t, err, ErrDailyLimitReached)
}

func TestGenerateDaily_CreatesNextRound_WhenUnderLimit(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithMax(repo, 5)
	ctx := context.Background()

	ds0, err := svc.GenerateDaily(ctx, 1, 10, user.A1)
	require.NoError(t, err)
	require.Equal(t, 0, ds0.Round)
	repo.answerAll(ds0.ID, 1)

	ds1, err := svc.GenerateDaily(ctx, 1, 10, user.A1)
	require.NoError(t, err)
	assert.Equal(t, 1, ds1.Round)
}

func TestGenerateDaily_ReturnsInProgressSet_WhenTasksUnanswered(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithMax(repo, 5)
	ctx := context.Background()

	ds0, err := svc.GenerateDaily(ctx, 1, 10, user.A1)
	require.NoError(t, err)
	// Do not answer any tasks
	ds0again, err := svc.GenerateDaily(ctx, 1, 10, user.A1)
	require.NoError(t, err)
	assert.Equal(t, ds0.ID, ds0again.ID, "should return the in-progress set, not create a new round")
}

func TestService_MaxRounds_ExposedForHandler(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithMax(repo, 7)
	assert.Equal(t, 7, svc.MaxRounds())
}
