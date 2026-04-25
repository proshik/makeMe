package words

import (
	"hash/fnv"
	"time"

	"makeMe/internal/domain/srs"
)

// NextSlot picks the next (word, mode) to show.
//
// Algorithm:
//   - Build the candidate list: every (word, mode) where the per-mode counter
//     is below ThresholdPerMode and the word is not yet learned. Words in
//     session.DeferredWordIDs are excluded entirely.
//   - If empty → session is done.
//   - Filter out the last-shown word (anti-repeat). If filtering empties the
//     list (e.g. only one unfinished word remains), keep the unfiltered list.
//   - Pick deterministically using a daily seed.
//
// `wordsByID` must contain every word in `session.WordIDs`.
func NextSlot(session *Session, wordsByID map[int64]*srs.Word, progress map[int64]Progress, now time.Time) (Slot, bool) {
	deferred := make(map[int64]bool, len(session.DeferredWordIDs))
	for _, id := range session.DeferredWordIDs {
		deferred[id] = true
	}

	all := buildCandidates(session.WordIDs, deferred, progress, 0)
	if len(all) == 0 {
		return Slot{}, true
	}

	candidates := all
	if session.LastWordID != 0 {
		filtered := buildCandidates(session.WordIDs, deferred, progress, session.LastWordID)
		if len(filtered) > 0 {
			candidates = filtered
		}
	}

	day := now.UTC().Format("2006-01-02")
	seed := slotSeed(session.ID, day, len(candidates))
	idx := int(seed % uint64(len(candidates)))
	chosen := candidates[idx]

	return Slot{Word: wordsByID[chosen.wordID], Mode: chosen.mode}, false
}

type candidate struct {
	wordID int64
	mode   Mode
}

// buildCandidates collects all (wordID, mode) pairs that still need work.
// Words in `deferred` are skipped. If excludeWordID != 0, that word is also
// skipped (anti-repeat).
func buildCandidates(wordIDs []int64, deferred map[int64]bool, progress map[int64]Progress, excludeWordID int64) []candidate {
	out := make([]candidate, 0, len(wordIDs)*4)
	for _, wid := range wordIDs {
		if wid == excludeWordID {
			continue
		}
		if deferred[wid] {
			continue
		}
		p := progress[wid] // zero-value Progress is fine: Counts==nil, Learned==false
		if p.Learned {
			continue
		}
		for _, mode := range AllModes {
			if p.Counts[mode] < ThresholdPerMode {
				out = append(out, candidate{wordID: wid, mode: mode})
			}
		}
	}
	return out
}

// slotSeed returns a deterministic uint64 derived from the session, the date,
// and the size of the candidate pool. Including the pool size means every
// answer (which mutates the pool) yields a different next slot.
func slotSeed(sessionID int64, day string, poolSize int) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[i] = byte(sessionID >> (i * 8))
	}
	h.Write(buf[:])
	h.Write([]byte(day))
	for i := 0; i < 8; i++ {
		buf[i] = byte(poolSize >> (i * 8))
	}
	h.Write(buf[:])
	return h.Sum64()
}
