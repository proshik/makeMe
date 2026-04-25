package words

import (
	"testing"
	"time"

	"makeMe/internal/domain/srs"
)

// makeProgress builds a Progress map shorthand for tests.
func makeProgress(counts ...map[Mode]int) map[int64]Progress {
	out := make(map[int64]Progress)
	for i, c := range counts {
		out[int64(i+1)] = Progress{UserID: 1, WordID: int64(i + 1), Counts: c}
	}
	return out
}

func makeWordsByID(ids ...int64) map[int64]*srs.Word {
	out := make(map[int64]*srs.Word)
	for _, id := range ids {
		w := &srs.Word{ID: id}
		out[id] = w
	}
	return out
}

func TestNextSlot_AllUnstarted(t *testing.T) {
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1, 2}}
	wordsByID := makeWordsByID(1, 2)
	progress := map[int64]Progress{}

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	slot, done := NextSlot(session, wordsByID, progress, now)
	if done {
		t.Fatalf("expected not done")
	}
	if slot.Word == nil {
		t.Fatalf("expected a word in slot")
	}
}

func TestNextSlot_Done(t *testing.T) {
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1}}
	wordsByID := makeWordsByID(1)
	progress := makeProgress(map[Mode]int{
		ModeEnRuMC: 2, ModeRuEnMC: 2, ModeEnRuTyping: 2, ModeRuEnTyping: 2,
	})
	progress[1] = Progress{UserID: 1, WordID: 1, Counts: progress[1].Counts, Learned: true}

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	_, done := NextSlot(session, wordsByID, progress, now)
	if !done {
		t.Fatalf("expected done when all words learned")
	}
}

func TestNextSlot_SkipsCompletedModes(t *testing.T) {
	// Word 1: en_ru_mc done, others pending.
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1}}
	wordsByID := makeWordsByID(1)
	progress := makeProgress(map[Mode]int{
		ModeEnRuMC: 2, ModeRuEnMC: 0, ModeEnRuTyping: 0, ModeRuEnTyping: 0,
	})

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	slot, done := NextSlot(session, wordsByID, progress, now)
	if done {
		t.Fatalf("expected not done; other 3 modes pending")
	}
	if slot.Mode == ModeEnRuMC {
		t.Fatalf("expected mode != en_ru_mc (already completed)")
	}
}

func TestNextSlot_AntiRepeat(t *testing.T) {
	// Two words available, lastWordID=1; expect word 2.
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1, 2}, LastWordID: 1}
	wordsByID := makeWordsByID(1, 2)
	progress := map[int64]Progress{}

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	slot, done := NextSlot(session, wordsByID, progress, now)
	if done {
		t.Fatalf("expected not done")
	}
	if slot.Word.ID == 1 {
		t.Fatalf("anti-repeat failed: got the same word as last")
	}
}

func TestNextSlot_AntiRepeat_SoloWordFallsBack(t *testing.T) {
	// Only word 1 has unfinished modes; lastWordID=1; should still return it.
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1, 2}, LastWordID: 1}
	wordsByID := makeWordsByID(1, 2)
	progress := makeProgress(
		map[Mode]int{ModeEnRuMC: 0, ModeRuEnMC: 0, ModeEnRuTyping: 0, ModeRuEnTyping: 0}, // wid=1
	)
	progress[2] = Progress{UserID: 1, WordID: 2, Learned: true,
		Counts: map[Mode]int{ModeEnRuMC: 2, ModeRuEnMC: 2, ModeEnRuTyping: 2, ModeRuEnTyping: 2},
	}

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	slot, done := NextSlot(session, wordsByID, progress, now)
	if done {
		t.Fatalf("expected not done; word 1 has unfinished modes")
	}
	if slot.Word.ID != 1 {
		t.Fatalf("expected fallback to word 1 (only candidate); got %d", slot.Word.ID)
	}
}

func TestNextSlot_SkipsDeferred(t *testing.T) {
	// Two words; word 1 is deferred, only word 2 should be considered.
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1, 2}, DeferredWordIDs: []int64{1}}
	wordsByID := makeWordsByID(1, 2)
	progress := map[int64]Progress{}

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	slot, done := NextSlot(session, wordsByID, progress, now)
	if done {
		t.Fatalf("expected not done; word 2 still has unfinished modes")
	}
	if slot.Word.ID != 2 {
		t.Fatalf("expected word 2 (deferred word 1 must be skipped), got %d", slot.Word.ID)
	}
}

func TestNextSlot_AllDeferred_Done(t *testing.T) {
	session := &Session{ID: 1, UserID: 1, WordIDs: []int64{1, 2}, DeferredWordIDs: []int64{1, 2}}
	wordsByID := makeWordsByID(1, 2)
	progress := map[int64]Progress{}

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	_, done := NextSlot(session, wordsByID, progress, now)
	if !done {
		t.Fatalf("expected done when every word in session is deferred")
	}
}

func TestSession_IsDeferred(t *testing.T) {
	s := &Session{DeferredWordIDs: []int64{5, 7, 9}}
	if !s.IsDeferred(7) {
		t.Fatalf("expected 7 to be deferred")
	}
	if s.IsDeferred(8) {
		t.Fatalf("expected 8 not to be deferred")
	}
}

func TestProgress_AllModesPassed(t *testing.T) {
	p := Progress{Counts: map[Mode]int{
		ModeEnRuMC: 2, ModeRuEnMC: 2, ModeEnRuTyping: 2, ModeRuEnTyping: 2,
	}}
	if !p.AllModesPassed() {
		t.Fatalf("expected all modes passed")
	}
	p.Counts[ModeRuEnTyping] = 1
	if p.AllModesPassed() {
		t.Fatalf("expected one mode short")
	}
}

func TestEncodeDecodeMode(t *testing.T) {
	for _, m := range AllModes {
		code := EncodeMode(m)
		decoded, ok := DecodeMode(string(code))
		if !ok || decoded != m {
			t.Fatalf("round-trip failed for %s: code=%s decoded=%s ok=%v", m, code, decoded, ok)
		}
	}
	if _, ok := DecodeMode("xxxx"); ok {
		t.Fatalf("expected unknown code to return ok=false")
	}
}
