package telegram

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
)

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
	level, catIdx, total, seed, ok := parseLearnPractice("learn:practice:A2:7:5:123456789")
	require.True(t, ok)
	assert.Equal(t, user.A2, level)
	assert.Equal(t, 7, catIdx)
	assert.Equal(t, 5, total)
	assert.Equal(t, int64(123456789), seed)
}

func TestParseLearnPractice_Invalid(t *testing.T) {
	// Old format without seed must now fail (forces clients to upgrade callbacks).
	_, _, _, _, ok := parseLearnPractice("learn:practice:A2:7:5")
	assert.False(t, ok)

	// Non-numeric seed.
	_, _, _, _, ok = parseLearnPractice("learn:practice:A2:7:5:abc")
	assert.False(t, ok)
}

func TestParseLearnQA(t *testing.T) {
	level, catIdx, qIdx, total, seed, optIdx, ok := parseLearnQA("learn:qa:C1:2:1:3:987654321:0")
	require.True(t, ok)
	assert.Equal(t, user.C1, level)
	assert.Equal(t, 2, catIdx)
	assert.Equal(t, 1, qIdx)
	assert.Equal(t, 3, total)
	assert.Equal(t, int64(987654321), seed)
	assert.Equal(t, 0, optIdx)
}

func TestParseLearnQA_Skip(t *testing.T) {
	// "🤷 Не знаю" encodes optIdx=-1.
	_, _, _, _, _, optIdx, ok := parseLearnQA("learn:qa:B1:0:0:5:42:-1")
	require.True(t, ok)
	assert.Equal(t, -1, optIdx)
}

func TestParseLearnQA_LongestCallback(t *testing.T) {
	// Verify the longest realistic callback (uint32 max seed = 4294967295,
	// 10 chars) stays within the 64-byte Telegram callback_data limit.
	s := fmt.Sprintf("learn:qa:%s:%d:%d:%d:%d:%d", "B1", 14, 19, 20, 4294967295, 3)
	assert.LessOrEqual(t, len(s), 64, "callback data must fit in 64 bytes")
}
