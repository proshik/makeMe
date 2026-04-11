package task

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

func testWord(id int64, en, ru, example string) *srs.Word {
	return &srs.Word{
		ID:      id,
		English: en,
		Russian: ru,
		Level:   user.A1,
		Example: example,
	}
}

func testDistractors(count int) []srs.Word {
	pairs := []struct{ en, ru string }{
		{"cat", "кот"},
		{"dog", "собака"},
		{"bird", "птица"},
		{"fish", "рыба"},
		{"tree", "дерево"},
	}
	result := make([]srs.Word, 0, count)
	for i := 0; i < count && i < len(pairs); i++ {
		result = append(result, srs.Word{
			ID:      int64(100 + i),
			English: pairs[i].en,
			Russian: pairs[i].ru,
			Level:   user.A1,
		})
	}
	return result
}

// ──────────────────────────────────────────────────────────────────────────────
// buildVocabEnRu
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildVocabEnRu_PromptContainsEnglishWord(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabEnRu(word, testDistractors(3))
	assert.Contains(t, task.Prompt, "apple")
}

func TestBuildVocabEnRu_AnswerIsRussian(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabEnRu(word, testDistractors(3))
	assert.Equal(t, "яблоко", task.Answer)
}

func TestBuildVocabEnRu_OptionsAreRussian(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabEnRu(word, testDistractors(3))
	require.Len(t, task.Options, 4)
	for _, opt := range task.Options {
		// All options should be Russian (no ASCII-only words in options)
		assert.True(t, containsCyrillic(opt), "expected Cyrillic option, got %q", opt)
	}
}

func TestBuildVocabEnRu_CorrectAnswerInOptions(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabEnRu(word, testDistractors(3))
	assert.Contains(t, task.Options, "яблоко")
}

func TestBuildVocabEnRu_MetadataSubtype(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabEnRu(word, testDistractors(3))
	assert.Equal(t, VocabSubtypeEnRu, task.Metadata["subtype"])
}

// ──────────────────────────────────────────────────────────────────────────────
// buildVocabRuEn
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildVocabRuEn_PromptIsRussianWord(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabRuEn(word, testDistractors(3))
	assert.Contains(t, task.Prompt, "яблоко")
	assert.NotContains(t, task.Prompt, "apple")
}

func TestBuildVocabRuEn_AnswerIsEnglish(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabRuEn(word, testDistractors(3))
	assert.Equal(t, "apple", task.Answer)
}

func TestBuildVocabRuEn_OptionsAreEnglish(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabRuEn(word, testDistractors(3))
	require.Len(t, task.Options, 4)
	for _, opt := range task.Options {
		assert.False(t, containsCyrillic(opt), "expected Latin option, got %q", opt)
	}
}

func TestBuildVocabRuEn_CorrectAnswerInOptions(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabRuEn(word, testDistractors(3))
	assert.Contains(t, task.Options, "apple")
}

func TestBuildVocabRuEn_MetadataSubtype(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "")
	task := buildVocabRuEn(word, testDistractors(3))
	assert.Equal(t, VocabSubtypeRuEn, task.Metadata["subtype"])
}

// ──────────────────────────────────────────────────────────────────────────────
// buildVocabFillBlank
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildVocabFillBlank_PromptContainsBlank(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "She ate an apple every day.")
	task := buildVocabFillBlank(word, testDistractors(3))
	assert.Contains(t, task.Prompt, "___")
}

func TestBuildVocabFillBlank_PromptDoesNotContainEnglishWord(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "She ate an apple every day.")
	task := buildVocabFillBlank(word, testDistractors(3))
	assert.NotContains(t, strings.ToLower(task.Prompt), "apple")
}

func TestBuildVocabFillBlank_AnswerIsEnglish(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "She ate an apple every day.")
	task := buildVocabFillBlank(word, testDistractors(3))
	assert.Equal(t, "apple", task.Answer)
}

func TestBuildVocabFillBlank_OptionsAreEnglish(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "She ate an apple every day.")
	task := buildVocabFillBlank(word, testDistractors(3))
	require.Len(t, task.Options, 4)
	for _, opt := range task.Options {
		assert.False(t, containsCyrillic(opt), "expected Latin option, got %q", opt)
	}
}

func TestBuildVocabFillBlank_CorrectAnswerInOptions(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "She ate an apple every day.")
	task := buildVocabFillBlank(word, testDistractors(3))
	assert.Contains(t, task.Options, "apple")
}

func TestBuildVocabFillBlank_FallsBackToEnRuWhenNoExample(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "") // no example
	task := buildVocabFillBlank(word, testDistractors(3))
	// Falls back to en_ru: answer is Russian, options are Russian
	assert.Equal(t, VocabSubtypeEnRu, task.Metadata["subtype"])
	assert.Equal(t, "яблоко", task.Answer)
}

func TestBuildVocabFillBlank_FallsBackWhenWordNotInExample(t *testing.T) {
	word := testWord(1, "ephemeral", "эфемерный", "This sentence has no matching word.")
	task := buildVocabFillBlank(word, testDistractors(3))
	assert.Equal(t, VocabSubtypeEnRu, task.Metadata["subtype"])
}

func TestBuildVocabFillBlank_MetadataSubtype(t *testing.T) {
	word := testWord(1, "apple", "яблоко", "She ate an apple every day.")
	task := buildVocabFillBlank(word, testDistractors(3))
	assert.Equal(t, VocabSubtypeFillBlank, task.Metadata["subtype"])
}

// ──────────────────────────────────────────────────────────────────────────────
// helpers
// ──────────────────────────────────────────────────────────────────────────────

func containsCyrillic(s string) bool {
	for _, r := range s {
		if r >= 0x0400 && r <= 0x04FF {
			return true
		}
	}
	return false
}
