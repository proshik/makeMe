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
func (f *fakeRepo) GetProgress(context.Context, int64, int64) (*Progress, error)                {
	return nil, nil
}
func (f *fakeRepo) BulkGetProgress(context.Context, int64, []int64) (map[int64]Progress, error) {
	return nil, nil
}
func (f *fakeRepo) IncProgress(context.Context, int64, int64, Mode) error                      { return nil }
func (f *fakeRepo) ResetProgress(context.Context, int64, int64, Mode) error                    { return nil }
func (f *fakeRepo) MarkLearned(context.Context, int64, int64) error                            { return nil }
func (f *fakeRepo) MarkAlreadyKnown(context.Context, int64, int64) error                       { return nil }
func (f *fakeRepo) CountInProgress(context.Context, int64) (int, error)                        { return 0, nil }
func (f *fakeRepo) CountLearnedViaWords(context.Context, int64) (int, error)                   { return 0, nil }
func (f *fakeRepo) CreateSession(context.Context, int64, time.Time, []int64) (*Session, error) {
	return nil, nil
}
func (f *fakeRepo) GetOpenSession(context.Context, int64) (*Session, error)            { return nil, nil }
func (f *fakeRepo) GetSessionByID(context.Context, int64) (*Session, error)            { return nil, nil }
func (f *fakeRepo) UpdateLastWord(context.Context, int64, int64) error                 { return nil }
func (f *fakeRepo) CompleteSession(context.Context, int64) error                       { return nil }
func (f *fakeRepo) IncSessionWordError(context.Context, int64, int64) error            { return nil }
func (f *fakeRepo) GetSessionWordError(context.Context, int64, int64) (int, error)     { return 0, nil }
func (f *fakeRepo) DeferSessionWord(context.Context, int64, int64) error               { return nil }

func mkWords(ids ...int64) []srs.Word {
	out := make([]srs.Word, len(ids))
	for i, id := range ids {
		out[i] = srs.Word{ID: id}
	}
	return out
}

// TestSelectWords_AscendingFromA1 — even though there are plenty of B1 words
// available, A1 must be exhausted first.
func TestSelectWords_AscendingFromA1(t *testing.T) {
	repo := &fakeRepo{
		fresh: map[user.Level][]srs.Word{
			user.A1: mkWords(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12),
			user.B1: mkWords(101, 102, 103, 104, 105),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 10 {
		t.Fatalf("expected 10 words, got %d", len(picked))
	}
	for _, w := range picked {
		if w.ID > 100 {
			t.Fatalf("got ID %d — expected only A1 ids (<=100), B1 should not appear while A1 has new words", w.ID)
		}
	}
}

// TestSelectWords_ClimbsWhenLevelExhausted — A1 has only 3 new words; the
// selector tops up with A2 and finally B1 in ascending order.
func TestSelectWords_ClimbsWhenLevelExhausted(t *testing.T) {
	repo := &fakeRepo{
		fresh: map[user.Level][]srs.Word{
			user.A1: mkWords(1, 2, 3),
			user.A2: mkWords(11, 12, 13, 14),
			user.B1: mkWords(21, 22, 23, 24, 25),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 10 {
		t.Fatalf("expected 10 words, got %d", len(picked))
	}
	gotIDs := make([]int64, len(picked))
	for i, w := range picked {
		gotIDs[i] = w.ID
	}
	wantIDs := []int64{1, 2, 3, 11, 12, 13, 14, 21, 22, 23}
	for i, want := range wantIDs {
		if gotIDs[i] != want {
			t.Fatalf("position %d: got %d, want %d (full got=%v)", i, gotIDs[i], want, gotIDs)
		}
	}
}

// TestSelectWords_InProgressFirst — words already started take priority
// over new ones, regardless of level.
func TestSelectWords_InProgressFirst(t *testing.T) {
	repo := &fakeRepo{
		inProgress: map[user.Level][]srs.Word{
			user.A2: mkWords(50, 51),
			user.B2: mkWords(60),
		},
		fresh: map[user.Level][]srs.Word{
			user.A1: mkWords(1, 2, 3, 4, 5, 6, 7, 8),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 10 {
		t.Fatalf("expected 10 words, got %d", len(picked))
	}
	// First 3 = in-progress (A2 then B2 ascending), then 7 new from A1.
	wantHeader := []int64{50, 51, 60}
	for i, want := range wantHeader {
		if picked[i].ID != want {
			t.Fatalf("position %d: got %d, want %d (in-progress should come first)", i, picked[i].ID, want)
		}
	}
	for i := 3; i < 10; i++ {
		if picked[i].ID > 100 {
			t.Fatalf("position %d: expected an A1 fresh word (id<100), got %d", i, picked[i].ID)
		}
	}
}

// TestSelectWords_UserLevelIgnored — same fixture, different "profile"
// inputs are no longer accepted by SelectWords; result is identical.
func TestSelectWords_UserLevelIgnored(t *testing.T) {
	repo := &fakeRepo{
		fresh: map[user.Level][]srs.Word{
			user.A1: mkWords(1, 2, 3, 4, 5, 6, 7, 8, 9, 10),
			user.C1: mkWords(900, 901, 902),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 10 {
		t.Fatalf("expected 10 words, got %d", len(picked))
	}
	for _, w := range picked {
		if w.ID >= 900 {
			t.Fatalf("C1 word %d leaked in even though A1 isn't exhausted", w.ID)
		}
	}
}

// TestSelectWords_PartialCoverage — fewer words available than SessionSize.
func TestSelectWords_PartialCoverage(t *testing.T) {
	repo := &fakeRepo{
		fresh: map[user.Level][]srs.Word{
			user.A1: mkWords(1, 2, 3),
			user.B1: mkWords(20, 21),
		},
	}
	picked, err := SelectWords(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 5 {
		t.Fatalf("expected 5 words (3 A1 + 2 B1), got %d", len(picked))
	}
}

// TestSelectWords_AllExhausted — no words at all → empty slice, no error.
func TestSelectWords_AllExhausted(t *testing.T) {
	repo := &fakeRepo{}
	picked, err := SelectWords(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(picked) != 0 {
		t.Fatalf("expected empty pick, got %d", len(picked))
	}
}
