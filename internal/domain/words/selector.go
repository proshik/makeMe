package words

import (
	"context"
	"fmt"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

// ascendingLevels is the order in which /words walks the dictionary: lowest
// CEFR first, climbing only when the previous level is exhausted (every word
// either learned or in progress). The user's profile level is intentionally
// ignored here — the goal is full coverage of the easier vocabulary first.
var ascendingLevels = user.AllLevels()

// SelectWords picks up to SessionSize words for a /words session.
//
// Order of preference:
//  1. In-progress words (started but not yet learned), walking levels A1→C2.
//  2. Brand-new words (no progress row, not in SM-2 either), walking levels A1→C2.
//
// We climb to the next level only after the current one is exhausted, so a
// user who hasn't yet seen all A1 keeps drilling A1 — even if their profile
// level is, say, B2. The "✅ Я уже знаю это слово" button lets them flag
// known words explicitly, which is the only way to retire an A1 word
// without going through the full 4-mode drill.
//
// If the entire dictionary is exhausted, fewer than SessionSize words may be
// returned. The caller renders what we got and treats an empty list as
// "session done".
func SelectWords(ctx context.Context, repo Repository, userID int64) ([]srs.Word, error) {
	picks := make([]srs.Word, 0, SessionSize)
	seen := make(map[int64]struct{}, SessionSize)

	add := func(ws []srs.Word) bool {
		for _, w := range ws {
			if len(picks) >= SessionSize {
				return true
			}
			if _, dup := seen[w.ID]; dup {
				continue
			}
			seen[w.ID] = struct{}{}
			picks = append(picks, w)
		}
		return len(picks) >= SessionSize
	}

	// Phase 1: in-progress, ascending.
	for _, lvl := range ascendingLevels {
		need := SessionSize - len(picks)
		if need <= 0 {
			break
		}
		ws, err := repo.GetInProgressWordsByLevel(ctx, userID, lvl, need)
		if err != nil {
			return nil, fmt.Errorf("in-progress %s: %w", lvl, err)
		}
		if add(ws) {
			break
		}
	}

	// Phase 2: brand-new words, ascending.
	for _, lvl := range ascendingLevels {
		need := SessionSize - len(picks)
		if need <= 0 {
			break
		}
		ws, err := repo.GetNewWordsExcludingProgress(ctx, userID, lvl, need)
		if err != nil {
			// Don't fail the whole pick if a level query fails — keep going.
			continue
		}
		if add(ws) {
			break
		}
	}

	return picks, nil
}
