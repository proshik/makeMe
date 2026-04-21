package telegram

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

func TestNextLevel(t *testing.T) {
	cases := []struct {
		in      user.Level
		wantOut user.Level
		wantOK  bool
	}{
		{user.A1, user.A2, true},
		{user.A2, user.B1, true},
		{user.B1, user.B2, true},
		{user.B2, user.C1, true},
		{user.C1, user.C2, true},
		{user.C2, "", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			got, ok := nextLevel(tc.in)
			assert.Equal(t, tc.wantOut, got)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestParseLearnCat(t *testing.T) {
	level, idx, ok := parseLearnCat("learn:cat:B1:3")
	require.True(t, ok)
	assert.Equal(t, user.B1, level)
	assert.Equal(t, 3, idx)
}

func TestParseLearnCat_Invalid(t *testing.T) {
	_, _, ok := parseLearnCat("learn:cat:B1")
	assert.False(t, ok)

	_, _, ok = parseLearnCat("learn:cat:B1:abc")
	assert.False(t, ok)
}

func TestParseLearnPractice(t *testing.T) {
	level, catIdx, total, ok := parseLearnPractice("learn:practice:A2:7:5")
	require.True(t, ok)
	assert.Equal(t, user.A2, level)
	assert.Equal(t, 7, catIdx)
	assert.Equal(t, 5, total)
}

func TestParseLearnQA(t *testing.T) {
	level, catIdx, qIdx, total, optIdx, ok := parseLearnQA("learn:qa:C1:2:1:3:0")
	require.True(t, ok)
	assert.Equal(t, user.C1, level)
	assert.Equal(t, 2, catIdx)
	assert.Equal(t, 1, qIdx)
	assert.Equal(t, 3, total)
	assert.Equal(t, 0, optIdx)
}

func TestParseLearnQA_LongestCallback(t *testing.T) {
	// Verify the longest realistic callback stays within 64-byte Telegram limit
	s := fmt.Sprintf("learn:qa:%s:%d:%d:%d:%d", "B1", 14, 3, 5, 2)
	assert.LessOrEqual(t, len(s), 64, "callback data must fit in 64 bytes")
}
