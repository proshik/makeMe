package assessment

import (
	"testing"

	"makeMe/internal/domain/user"
)

func TestAdjustLevel(t *testing.T) {
	tests := []struct {
		name    string
		current user.Level
		correct bool
		want    user.Level
	}{
		{"correct from B1 goes up", user.B1, true, user.B2},
		{"wrong from B1 goes down", user.B1, false, user.A2},
		{"correct from C2 stays", user.C2, true, user.C2},
		{"wrong from A1 stays", user.A1, false, user.A1},
		{"correct from A1 goes up", user.A1, true, user.A2},
		{"wrong from C2 goes down", user.C2, false, user.C1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustLevel(tt.current, tt.correct)
			if got != tt.want {
				t.Errorf("adjustLevel(%s, %v) = %s, want %s", tt.current, tt.correct, got, tt.want)
			}
		})
	}
}

func TestDetermineLevel(t *testing.T) {
	tests := []struct {
		name  string
		stats map[string]LevelStats
		want  user.Level
	}{
		{
			name:  "all correct at B1",
			stats: map[string]LevelStats{"B1": {Correct: 5, Total: 5}},
			want:  user.B1,
		},
		{
			name:  "correct at multiple levels picks highest",
			stats: map[string]LevelStats{"A1": {3, 3}, "A2": {2, 3}, "B1": {1, 2}},
			want:  user.B1,
		},
		{
			name:  "below 50% doesn't count",
			stats: map[string]LevelStats{"A1": {3, 3}, "A2": {1, 3}},
			want:  user.A1,
		},
		{
			name:  "empty stats defaults to A1",
			stats: map[string]LevelStats{},
			want:  user.A1,
		},
		{
			name:  "all wrong defaults to A1",
			stats: map[string]LevelStats{"B1": {0, 5}},
			want:  user.A1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineLevel(tt.stats)
			if got != tt.want {
				t.Errorf("determineLevel() = %s, want %s", got, tt.want)
			}
		})
	}
}
