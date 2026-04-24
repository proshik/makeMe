package task

import (
	"time"

	"makeMe/internal/domain/user"
)

type TaskType string

const (
	TypeVocab   TaskType = "vocab"
	TypeGrammar TaskType = "grammar"
)

// Vocab subtype constants stored in Task.Metadata["subtype"].
const (
	VocabSubtypeEnRu      = "en_ru"
	VocabSubtypeRuEn      = "ru_en"
	VocabSubtypeFillBlank = "fill_blank"
)

type DailySet struct {
	ID        int64
	UserID    int64
	GroupID   int64
	Date      time.Time
	CreatedAt time.Time
	Round     int
	Tasks     []Task
}

type Task struct {
	ID         int64
	DailySetID int64
	GroupID    int64 // populated only when loaded via GetTaskByID
	Type       TaskType
	Level      user.Level
	Prompt     string
	Options    []string
	Answer     string
	Metadata   map[string]any
	Position   int
}

type TaskResult struct {
	ID         int64
	TaskID     int64
	UserID     int64
	Response   string
	Correct    bool
	Score      int
	Skipped    bool
	DurationMs int
	CreatedAt  time.Time
}
