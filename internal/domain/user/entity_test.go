package user

import "testing"

func TestAllLevelsIsAscending(t *testing.T) {
	got := AllLevels()
	want := []Level{A1, A2, B1, B2, C1, C2}

	if len(got) != len(want) {
		t.Fatalf("got %d levels, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// The canonical slice must not be reachable through the accessor: a caller that
// sorted or truncated it would silently reorder the CEFR ladder for the whole
// process — level promotion, word selection and every progress display at once.
func TestAllLevelsReturnsACopy(t *testing.T) {
	first := AllLevels()
	first[0] = C2

	if second := AllLevels(); second[0] != A1 {
		t.Fatalf("mutating the result changed the canonical order: %v", second)
	}
}

func TestLevelIndex(t *testing.T) {
	for i, l := range AllLevels() {
		if got := l.Index(); got != i {
			t.Errorf("%s.Index() = %d, want %d", l, got, i)
		}
	}
	if got := Level("").Index(); got != -1 {
		t.Errorf("empty level Index() = %d, want -1", got)
	}
	if got := Level("Z9").Index(); got != -1 {
		t.Errorf("unknown level Index() = %d, want -1", got)
	}
}

func TestLevelNext(t *testing.T) {
	cases := []struct {
		from Level
		want Level
		ok   bool
	}{
		{A1, A2, true},
		{B2, C1, true},
		{C2, "", false}, // the top of the ladder has nowhere to go
		{"", "", false},
		{"Z9", "", false},
	}
	for _, tc := range cases {
		got, ok := tc.from.Next()
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q.Next() = (%q, %v), want (%q, %v)", tc.from, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLevelPrev(t *testing.T) {
	cases := []struct {
		from Level
		want Level
		ok   bool
	}{
		{A2, A1, true},
		{C2, C1, true},
		{A1, "", false}, // the bottom likewise
		{"", "", false},
		{"Z9", "", false},
	}
	for _, tc := range cases {
		got, ok := tc.from.Prev()
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q.Prev() = (%q, %v), want (%q, %v)", tc.from, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLevelAtLeast(t *testing.T) {
	if !B2.AtLeast(A1) {
		t.Error("B2 should be at least A1")
	}
	if !B2.AtLeast(B2) {
		t.Error("a level is at least itself")
	}
	if B1.AtLeast(C1) {
		t.Error("B1 is not at least C1")
	}
	// An unset level must not light up any step of a progress scale.
	if Level("").AtLeast(A1) {
		t.Error("an unknown level should be below everything")
	}
}
