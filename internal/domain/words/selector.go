package words

import (
	"context"
	"fmt"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

// nextLevel returns the level above l, or "" if l is C2.
func nextLevel(l user.Level) user.Level {
	levels := []user.Level{user.A1, user.A2, user.B1, user.B2, user.C1, user.C2}
	for i, lv := range levels {
		if lv == l && i+1 < len(levels) {
			return levels[i+1]
		}
	}
	return ""
}

// SelectWords picks SessionSize words for a /words session, prioritizing
// in-progress words (already started but not learned) over new ones, with
// a 50/50 split between current and next level. C2 users get all 10 from
// current level.
//
// Each level's quota is HalfSession (=5). For each level we first take up
// to 5 in-progress, then top up with brand-new words (no progress row,
// not yet in SM-2 either). If even that doesn't fill the quota at one level,
// the shortfall is taken from the other level so the total is still up to
// SessionSize. If the entire dictionary at the user's reach is exhausted,
// fewer than SessionSize words may be returned — the caller renders what
// we got and reports session done if zero.
func SelectWords(ctx context.Context, repo Repository, userID int64, current user.Level) ([]srs.Word, error) {
	next := nextLevel(current)

	currentWords, err := pickForLevel(ctx, repo, userID, current, HalfSession)
	if err != nil {
		return nil, fmt.Errorf("pick current: %w", err)
	}

	var nextWords []srs.Word
	if next != "" {
		nextWords, err = pickForLevel(ctx, repo, userID, next, HalfSession)
		if err != nil {
			return nil, fmt.Errorf("pick next: %w", err)
		}
	}

	// Top up shortfalls cross-level so we still get up to SessionSize.
	currentShort := HalfSession - len(currentWords)
	nextShort := HalfSession - len(nextWords)
	if next == "" {
		// C2: try to take 10 from current alone.
		if currentShort > 0 {
			extra, err := pickForLevel(ctx, repo, userID, current, HalfSession+currentShort)
			if err == nil {
				currentWords = mergeUnique(currentWords, extra, HalfSession*2)
			}
		}
	} else {
		if currentShort > 0 && len(nextWords) >= HalfSession {
			extra, err := pickForLevel(ctx, repo, userID, next, currentShort)
			if err == nil {
				nextWords = mergeUnique(nextWords, extra, HalfSession+currentShort)
			}
		}
		if nextShort > 0 && len(currentWords) >= HalfSession {
			extra, err := pickForLevel(ctx, repo, userID, current, nextShort)
			if err == nil {
				currentWords = mergeUnique(currentWords, extra, HalfSession+nextShort)
			}
		}
	}

	out := make([]srs.Word, 0, SessionSize)
	out = append(out, currentWords...)
	out = append(out, nextWords...)
	return out, nil
}

// pickForLevel returns up to `limit` words for one level: in-progress first,
// then new words to fill the remainder.
func pickForLevel(ctx context.Context, repo Repository, userID int64, level user.Level, limit int) ([]srs.Word, error) {
	if limit <= 0 {
		return nil, nil
	}
	inProgress, err := repo.GetInProgressWordsByLevel(ctx, userID, level, limit)
	if err != nil {
		return nil, err
	}
	got := len(inProgress)
	if got >= limit {
		return inProgress[:limit], nil
	}
	fresh, err := repo.GetNewWordsExcludingProgress(ctx, userID, level, limit-got)
	if err != nil {
		// Don't fail — some in-progress is better than none.
		return inProgress, nil
	}
	return append(inProgress, fresh...), nil
}

// mergeUnique appends `add` items to `base` skipping word IDs already present,
// stopping at `cap`.
func mergeUnique(base, add []srs.Word, cap int) []srs.Word {
	seen := make(map[int64]struct{}, len(base))
	for _, w := range base {
		seen[w.ID] = struct{}{}
	}
	for _, w := range add {
		if len(base) >= cap {
			break
		}
		if _, dup := seen[w.ID]; dup {
			continue
		}
		seen[w.ID] = struct{}{}
		base = append(base, w)
	}
	return base
}
