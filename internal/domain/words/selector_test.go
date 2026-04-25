package words

import (
	"context"
	"testing"
	"time"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

// fakeRepo is a stub that satisfies words.Repository for unit-testing the
// selector logic without a database.
type fakeRepo struct {
	inProgress map[user.Level][]srs.Word
	fresh      map[user.Level][]srs.Word
}

func (f *fakeRepo) GetInProgressWordsByLevel(_ context.Context, _ int64, level user.Level, limit int) ([]srs.Word, error) {
	ws := f.inProgress[level]
	if limit < len(ws) {
		ws = ws[:limit]
	}
	return ws, nil
}

func (f *fakeRepo) GetNewWordsExcludingProgress(_ context.Context, _ int64, level user.Level, limit int) ([]srs.Word, error) {
	ws := f.fresh[level]
	if limit < len(ws) {
		ws = ws[:limit]
	}
	return ws, nil
}

// Methods below are unused by selector_test but required by the interface.
func (f *fakeRepo) GetProgress(context.Context, int64, int64) (*Progress, error)                   { return nil, nil }
func (f *fakeRepo) BulkGetProgress(context.Context, int64, []int64) (map[int64]Progress, error)    { return nil, nil }
func (f *fakeRepo) IncProgress(context.Context, int64, int64, Mode) error                          { return nil }
func (f *fakeRepo) ResetProgress(context.Context, int64, int64, Mode) error                        { return nil }
func (f *fakeRepo) MarkLearned(context.Context, int64, int64) error                                { return nil }
func (f *fakeRepo) MarkAlreadyKnown(context.Context, int64, int64) error                           { return nil }
func (f *fakeRepo) CountInProgress(context.Context, int64) (int, error)                            { return 0, nil }
func (f *fakeRepo) CountLearnedViaWords(context.Context, int64) (int, error)                       { return 0, nil }
func (f *fakeRepo) CreateSession(context.Context, int64, time.Time, []int64) (*Session, error)     { return nil, nil }
func (f *fakeRepo) GetOpenSession(context.Context, int64) (*Session, error)                        { return nil, nil }
func (f *fakeRepo) GetSessionByID(context.Context, int64) (*Session, error)                        { return nil, nil }
func (f *fakeRepo) UpdateLastWord(context.Context, int64, int64) error                             { return nil }
func (f *fakeRepo) CompleteSession(context.Context, int64) error                                   { return nil }
func (f *fakeRepo) IncSessionWordError(context.Context, int64, int64) error                        { return nil }
func (f *fakeRepo) GetSessionWordError(context.Context, int64, int64) (int, error)                 { return 0, nil }
func (f *fakeRepo) DeferSessionWord(context.Context, int64, int64) error                           { return nil }

func mkWords(prefix string, ids ...int64) []srs.Word {
	out := make([]srs.Word, len(ids))
	for i, id := range ids {
		out[i] = srs.Word{ID: id, English: prefix + "_" + string(rune('a'+i))}
	}
	return out
}

func TestSelectWords_HappyPath(t *testing.T) {
	// 5 in-progress at B1, 5 fresh at B2 → 5 + 5 = 10 total.
	repo := &fakeRepo{
		inProgress: map[user.Level][]srs.Word{
			user.B1: mkWords("ip", 1, 2, 3, 4, 5),
		},
		fresh: map[user.Level][]srs.Word{
			user.B1: mkWords("new1", 6, 7, 8, 9, 10),
			user.B2: mkWords("new2", 11, 12, 13, 14, 15),
		},
	}

	picked, err := SelectWords(context.Background(), repo, 1, user.B1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 10 {
		t.Fatalf("expected 10 words, got %d", len(picked))
	}
	// First 5 should be in-progress at B1 (priority).
	for i := 0; i < 5; i++ {
		if picked[i].ID != int64(i+1) {
			t.Fatalf("position %d: expected in-progress B1 word ID %d, got %d", i, i+1, picked[i].ID)
		}
	}
	// Next 5 should come from B2 (next level).
	for i := 5; i < 10; i++ {
		want := int64(11 + (i - 5))
		if picked[i].ID != want {
			t.Fatalf("position %d: expected next-level B2 word ID %d, got %d", i, want, picked[i].ID)
		}
	}
}

func TestSelectWords_C2_NoNextLevel(t *testing.T) {
	// C2 user: should fill all 10 from C2.
	repo := &fakeRepo{
		fresh: map[user.Level][]srs.Word{
			user.C2: mkWords("c2", 1, 2, 3, 4, 5, 6, 7, 8, 9, 10),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1, user.C2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) == 0 {
		t.Fatalf("expected words from current level")
	}
	for _, w := range picked {
		if w.ID < 1 || w.ID > 10 {
			t.Fatalf("unexpected word ID %d (should be from C2 fresh)", w.ID)
		}
	}
}

func TestSelectWords_PartialCoverage(t *testing.T) {
	// Only 3 fresh at B1, 5 fresh at B2: should still return 8 (no error).
	repo := &fakeRepo{
		fresh: map[user.Level][]srs.Word{
			user.B1: mkWords("b1", 1, 2, 3),
			user.B2: mkWords("b2", 4, 5, 6, 7, 8),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1, user.B1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) == 0 {
		t.Fatalf("expected at least the available words")
	}
}

func TestNextLevel(t *testing.T) {
	cases := []struct {
		in   user.Level
		want user.Level
	}{
		{user.A1, user.A2},
		{user.A2, user.B1},
		{user.B1, user.B2},
		{user.B2, user.C1},
		{user.C1, user.C2},
		{user.C2, ""},
	}
	for _, tc := range cases {
		if got := nextLevel(tc.in); got != tc.want {
			t.Fatalf("nextLevel(%s): got %s, want %s", tc.in, got, tc.want)
		}
	}
}
