package telegram

import (
	"strings"
	"testing"

	"makeMe/internal/domain/words"
)

func TestFormatLevelProgress_Empty(t *testing.T) {
	if got := formatLevelProgress(nil); got != "" {
		t.Fatalf("expected empty string for nil input, got %q", got)
	}
	if got := formatLevelProgress([]words.LevelProgress{}); got != "" {
		t.Fatalf("expected empty string for empty slice, got %q", got)
	}
}

func TestFormatLevelProgress_AllZeroTotal(t *testing.T) {
	in := []words.LevelProgress{
		{Level: "A1", Learned: 0, Total: 0},
		{Level: "A2", Learned: 0, Total: 0},
	}
	if got := formatLevelProgress(in); got != "" {
		t.Fatalf("expected empty string when all totals are 0, got %q", got)
	}
}

func TestFormatLevelProgress_AllZeroLearned(t *testing.T) {
	// Brand-new user: corpus is populated at every level, but they haven't
	// learned anything yet. The block should disappear entirely so /profile
	// and /words intro stay clean until the user earns their first row.
	in := []words.LevelProgress{
		{Level: "A1", Learned: 0, Total: 120},
		{Level: "A2", Learned: 0, Total: 180},
		{Level: "B1", Learned: 0, Total: 300},
	}
	if got := formatLevelProgress(in); got != "" {
		t.Fatalf("expected empty string when learned=0 at every level, got %q", got)
	}
}

func TestFormatLevelProgress_HappyPath(t *testing.T) {
	in := []words.LevelProgress{
		{Level: "A1", Learned: 12, Total: 120},
		{Level: "A2", Learned: 5, Total: 180},
		{Level: "B1", Learned: 0, Total: 300}, // dropped — not started
	}
	got := formatLevelProgress(in)

	expected := "📊 По уровням:\nA1: 12/120 (10%)\nA2: 5/180 (3%)\n"
	if got != expected {
		t.Fatalf("unexpected output:\nwant:\n%s\ngot:\n%s", expected, got)
	}
}

func TestFormatLevelProgress_SkipsZeroTotalRows(t *testing.T) {
	in := []words.LevelProgress{
		{Level: "A1", Learned: 12, Total: 120},
		{Level: "B2", Learned: 0, Total: 0},
		{Level: "C1", Learned: 0, Total: 0},
	}
	got := formatLevelProgress(in)

	if strings.Contains(got, "B2") || strings.Contains(got, "C1") {
		t.Fatalf("levels with Total=0 must be hidden, got %q", got)
	}
	if !strings.Contains(got, "A1: 12/120 (10%)") {
		t.Fatalf("A1 row missing, got %q", got)
	}
}

func TestFormatLevelProgress_SkipsZeroLearnedRows(t *testing.T) {
	in := []words.LevelProgress{
		{Level: "A1", Learned: 9, Total: 292},
		{Level: "A2", Learned: 6, Total: 200},
		{Level: "B1", Learned: 0, Total: 166},
		{Level: "B2", Learned: 0, Total: 146},
	}
	got := formatLevelProgress(in)

	if strings.Contains(got, "B1") || strings.Contains(got, "B2") {
		t.Fatalf("levels the user hasn't started must be hidden, got %q", got)
	}
	if !strings.Contains(got, "A1: 9/292 (3%)") || !strings.Contains(got, "A2: 6/200 (3%)") {
		t.Fatalf("started levels missing, got %q", got)
	}
}

func TestFormatLevelProgress_PercentRounding(t *testing.T) {
	cases := []struct {
		learned int
		total   int
		want    string // "(N%)" segment
	}{
		{1, 100, "(1%)"},
		{1, 3, "(33%)"}, // 33.33 → 33
		{2, 3, "(67%)"}, // 66.66 → 67
		{99, 100, "(99%)"},
		{100, 100, "(100%)"},
	}
	for _, c := range cases {
		got := formatLevelProgress([]words.LevelProgress{
			{Level: "A1", Learned: c.learned, Total: c.total},
		})
		if !strings.Contains(got, c.want) {
			t.Errorf("learned=%d total=%d: want %q in output, got %q", c.learned, c.total, c.want, got)
		}
	}
}

func TestFormatLevelProgress_TrailingNewline(t *testing.T) {
	got := formatLevelProgress([]words.LevelProgress{
		{Level: "A1", Learned: 1, Total: 10},
	})
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline so callers can append cleanly, got %q", got)
	}
}
