package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"makeMe/internal/domain/task"
)

func TestBuildIncorrectFeedback_GrammarWithExplanation(tt *testing.T) {
	tsk := &task.Task{
		Type:   task.TypeGrammar,
		Answer: "is",
		Metadata: map[string]any{
			"explanation": "Используй «is» с he/she/it в Present Simple.",
		},
	}
	got := buildIncorrectFeedback(tsk)
	assert.Contains(tt, got, "is")
	assert.Contains(tt, got, "💡")
	assert.Contains(tt, got, "Используй «is»")
}

func TestBuildIncorrectFeedback_GrammarNoExplanation(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeGrammar,
		Answer:   "went",
		Metadata: map[string]any{},
	}
	got := buildIncorrectFeedback(tsk)
	assert.Contains(tt, got, "went")
	assert.NotContains(tt, got, "💡")
}

func TestBuildIncorrectFeedback_GrammarNilMetadata(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeGrammar,
		Answer:   "went",
		Metadata: nil,
	}
	got := buildIncorrectFeedback(tsk)
	assert.Contains(tt, got, "went")
	assert.NotContains(tt, got, "💡")
}

func TestBuildIncorrectFeedback_VocabNoExplanation(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeVocab,
		Answer:   "яблоко",
		Metadata: nil,
	}
	got := buildIncorrectFeedback(tsk)
	assert.Contains(tt, got, "яблоко")
	assert.NotContains(tt, got, "💡")
}
