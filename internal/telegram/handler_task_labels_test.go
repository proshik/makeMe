package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"makeMe/internal/domain/task"
)

func TestVocabTaskLabel_EnRu(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeVocab,
		Metadata: map[string]any{"subtype": task.VocabSubtypeEnRu},
	}
	assert.Equal(tt, "📖 Перевод слов", vocabTaskLabel(tsk))
}

func TestVocabTaskLabel_RuEn(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeVocab,
		Metadata: map[string]any{"subtype": task.VocabSubtypeRuEn},
	}
	assert.Equal(tt, "📖 Перевод слов", vocabTaskLabel(tsk))
}

func TestVocabTaskLabel_FillBlank(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeVocab,
		Metadata: map[string]any{"subtype": task.VocabSubtypeFillBlank},
	}
	assert.Equal(tt, "📝 Заполни пропуск", vocabTaskLabel(tsk))
}

func TestVocabTaskLabel_NoSubtype(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeVocab,
		Metadata: map[string]any{},
	}
	assert.Equal(tt, "📖 Словарь", vocabTaskLabel(tsk))
}

func TestVocabTaskLabel_NilMetadata(tt *testing.T) {
	tsk := &task.Task{
		Type:     task.TypeVocab,
		Metadata: nil,
	}
	assert.Equal(tt, "📖 Словарь", vocabTaskLabel(tsk))
}
