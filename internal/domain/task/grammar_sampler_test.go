package task

import (
	"reflect"
	"testing"
)

func mkQ(id int64, sub string) GrammarQuestion {
	return GrammarQuestion{ID: id, Subcategory: sub}
}

func subs(out []GrammarQuestion) map[string]int {
	m := map[string]int{}
	for _, q := range out {
		m[q.Subcategory]++
	}
	return m
}

func TestStratifiedSample_EmptyInput(t *testing.T) {
	got := StratifiedSample(nil, 42, 20)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestStratifiedSample_ZeroLimit(t *testing.T) {
	qs := []GrammarQuestion{mkQ(1, "affirmative")}
	got := StratifiedSample(qs, 42, 0)
	if len(got) != 0 {
		t.Fatalf("expected empty for zero limit, got %d", len(got))
	}
}

func TestStratifiedSample_FewerThanLimit(t *testing.T) {
	qs := []GrammarQuestion{
		mkQ(1, "affirmative"),
		mkQ(2, "negative"),
		mkQ(3, "interrogative"),
	}
	got := StratifiedSample(qs, 42, 20)
	if len(got) != 3 {
		t.Fatalf("expected all 3 questions, got %d", len(got))
	}
	if !subsContainsAll(subs(got), []string{"affirmative", "negative", "interrogative"}) {
		t.Fatalf("expected every subcategory present, got %v", subs(got))
	}
}

func TestStratifiedSample_CoversAllSubcategoriesBeforeRepeating(t *testing.T) {
	qs := []GrammarQuestion{}
	for i := int64(1); i <= 10; i++ {
		qs = append(qs, mkQ(i, "affirmative"))
	}
	for i := int64(11); i <= 15; i++ {
		qs = append(qs, mkQ(i, "negative"))
	}
	for i := int64(16); i <= 18; i++ {
		qs = append(qs, mkQ(i, "interrogative"))
	}

	got := StratifiedSample(qs, 42, 9)
	counts := subs(got)
	if counts["affirmative"] != 3 || counts["negative"] != 3 || counts["interrogative"] != 3 {
		t.Fatalf("expected even round-robin distribution (3/3/3), got %v", counts)
	}
}

func TestStratifiedSample_DistributesLeftoverToLargerBuckets(t *testing.T) {
	qs := []GrammarQuestion{}
	for i := int64(1); i <= 15; i++ {
		qs = append(qs, mkQ(i, "affirmative"))
	}
	for i := int64(16); i <= 20; i++ {
		qs = append(qs, mkQ(i, "negative"))
	}
	for i := int64(21); i <= 23; i++ {
		qs = append(qs, mkQ(i, "interrogative"))
	}

	got := StratifiedSample(qs, 42, 20)
	counts := subs(got)
	if len(got) != 20 {
		t.Fatalf("expected 20 results, got %d", len(got))
	}
	if counts["interrogative"] != 3 {
		t.Fatalf("expected all 3 interrogative used, got %d", counts["interrogative"])
	}
	if counts["negative"] != 5 {
		t.Fatalf("expected all 5 negative used, got %d", counts["negative"])
	}
	if counts["affirmative"] != 12 {
		t.Fatalf("expected 12 affirmative (20-5-3), got %d", counts["affirmative"])
	}
}

func TestStratifiedSample_DeterministicSameSeed(t *testing.T) {
	qs := []GrammarQuestion{}
	for i := int64(1); i <= 10; i++ {
		qs = append(qs, mkQ(i, "affirmative"))
	}
	for i := int64(11); i <= 15; i++ {
		qs = append(qs, mkQ(i, "negative"))
	}

	a := StratifiedSample(qs, 12345, 8)
	b := StratifiedSample(qs, 12345, 8)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed should give identical result: %v vs %v", idsOf(a), idsOf(b))
	}
}

func TestStratifiedSample_DifferentSeedsDiffer(t *testing.T) {
	qs := []GrammarQuestion{}
	for i := int64(1); i <= 20; i++ {
		qs = append(qs, mkQ(i, "affirmative"))
	}
	a := StratifiedSample(qs, 1, 10)
	b := StratifiedSample(qs, 2, 10)
	if reflect.DeepEqual(idsOf(a), idsOf(b)) {
		t.Fatalf("different seeds should produce different order (got same): %v", idsOf(a))
	}
}

func TestStratifiedSample_MissingSubcategoryTreatedAsGeneral(t *testing.T) {
	qs := []GrammarQuestion{
		{ID: 1, Subcategory: ""},
		{ID: 2, Subcategory: ""},
		{ID: 3, Subcategory: "negative"},
	}
	got := StratifiedSample(qs, 42, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
}

func TestLearnSessionSeed_Deterministic(t *testing.T) {
	a := LearnSessionSeed(42, "Past Simple", "2026-04-24")
	b := LearnSessionSeed(42, "Past Simple", "2026-04-24")
	if a != b {
		t.Fatalf("same inputs should produce same seed: %d vs %d", a, b)
	}
}

func TestLearnSessionSeed_ChangesWithInputs(t *testing.T) {
	base := LearnSessionSeed(42, "Past Simple", "2026-04-24")
	if LearnSessionSeed(43, "Past Simple", "2026-04-24") == base {
		t.Fatalf("different user should change seed")
	}
	if LearnSessionSeed(42, "Present Simple", "2026-04-24") == base {
		t.Fatalf("different category should change seed")
	}
	if LearnSessionSeed(42, "Past Simple", "2026-04-25") == base {
		t.Fatalf("different day should change seed")
	}
}

func idsOf(qs []GrammarQuestion) []int64 {
	ids := make([]int64, len(qs))
	for i, q := range qs {
		ids[i] = q.ID
	}
	return ids
}

func subsContainsAll(m map[string]int, keys []string) bool {
	for _, k := range keys {
		if m[k] == 0 {
			return false
		}
	}
	return true
}
