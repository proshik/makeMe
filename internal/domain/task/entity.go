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

type DailySet struct {
	ID        int64
	UserID    int64
	GroupID   int64
	Date      time.Time
	CreatedAt time.Time
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
	DurationMs int
	CreatedAt  time.Time
}
