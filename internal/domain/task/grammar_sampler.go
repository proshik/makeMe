package task

import (
	"math/rand/v2"
	"sort"
)

// StratifiedSample returns up to limit questions chosen round-robin from
// per-subcategory buckets, so the resulting set covers as many subtypes
// (affirmative/negative/interrogative/…) as possible. Within each bucket
// questions are shuffled deterministically using the given seed, so the same
// seed always yields the same order (required because /learn re-derives the
// question list on every callback).
func StratifiedSample(qs []GrammarQuestion, seed int64, limit int) []GrammarQuestion {
	if limit <= 0 || len(qs) == 0 {
		return nil
	}

	buckets := make(map[string][]GrammarQuestion)
	for _, q := range qs {
		sub := q.Subcategory
		if sub == "" {
			sub = "general"
		}
		buckets[sub] = append(buckets[sub], q)
	}

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9E3779B97F4A7C15))
	for _, k := range keys {
		b := buckets[k]
		rng.Shuffle(len(b), func(i, j int) { b[i], b[j] = b[j], b[i] })
		buckets[k] = b
	}

	out := make([]GrammarQuestion, 0, min(limit, len(qs)))
	pos := make(map[string]int, len(keys))
	for len(out) < limit {
		progressed := false
		for _, k := range keys {
			if len(out) >= limit {
				break
			}
			b := buckets[k]
			p := pos[k]
			if p < len(b) {
				out = append(out, b[p])
				pos[k] = p + 1
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	return out
}
