// Package words implements the active-recall learning mode behind /words.
//
// Each word is drilled in 4 modes (EN→RU MC, RU→EN MC, EN→RU typing, RU→EN
// typing). When the user has answered correctly twice in every mode the word
// is promoted to the SM-2 schedule (review_cards) where /tasks picks it up
// for long-term retention.
//
// /words sessions do NOT contribute to scoring — that is the single behavioral
// difference vs /tasks.
package words

import (
	"time"

	"makeMe/internal/domain/srs"
)

// Mode identifies one of the 4 drill modes for a single word.
type Mode string

const (
	ModeEnRuMC     Mode = "en_ru_mc"
	ModeRuEnMC     Mode = "ru_en_mc"
	ModeEnRuTyping Mode = "en_ru_typing"
	ModeRuEnTyping Mode = "ru_en_typing"
)

// AllModes is the canonical iteration order for slot building.
var AllModes = [4]Mode{ModeEnRuMC, ModeRuEnMC, ModeEnRuTyping, ModeRuEnTyping}

const (
	// ThresholdPerMode is the number of correct answers per mode required to mark
	// the mode as completed. Wrong answer resets the per-mode counter to 0.
	ThresholdPerMode = 2
	// SessionSize is the number of words selected for one /words session.
	SessionSize = 10
	// HalfSession is the per-level quota (current + next level).
	HalfSession = 5
	// PendingTTL is how long a typing prompt waits for a reply before expiring.
	PendingTTL = 15 * time.Minute
	// DeferThreshold is the number of session-level errors on a word above
	// which the "defer until tomorrow" affordance is offered to the user.
	// The button appears starting from the (DeferThreshold + 1)-th wrong answer.
	DeferThreshold = 5
)

// Progress holds the per-mode counters and learned status for a single user/word.
type Progress struct {
	UserID    int64
	WordID    int64
	Counts    map[Mode]int
	Learned   bool
	LearnedAt *time.Time
	UpdatedAt time.Time
}

// AllModesPassed reports whether every mode has reached the threshold.
func (p Progress) AllModesPassed() bool {
	for _, m := range AllModes {
		if p.Counts[m] < ThresholdPerMode {
			return false
		}
	}
	return true
}

// Slot is a single (word, mode) pair to be presented to the user.
type Slot struct {
	Word *srs.Word
	Mode Mode
}

// IsTyping reports whether the slot's mode requires a typed text reply.
func (s Slot) IsTyping() bool {
	return s.Mode == ModeEnRuTyping || s.Mode == ModeRuEnTyping
}

// IsMC reports whether the slot's mode is multiple-choice.
func (s Slot) IsMC() bool {
	return s.Mode == ModeEnRuMC || s.Mode == ModeRuEnMC
}

// Session captures the 10 selected words for the day plus anti-repeat cursor
// and per-session error tracking.
type Session struct {
	ID              int64
	UserID          int64
	StartedOn       time.Time
	WordIDs         []int64
	LastWordID      int64 // 0 when no slot has been shown yet
	DeferredWordIDs []int64
	CompletedAt     *time.Time
}

// IsDeferred reports whether the given word ID is in DeferredWordIDs.
func (s *Session) IsDeferred(wordID int64) bool {
	for _, id := range s.DeferredWordIDs {
		if id == wordID {
			return true
		}
	}
	return false
}

// Outcome is returned after each answer is recorded.
type Outcome struct {
	Correct         bool
	BecameLearned   bool // word transitioned to learned just now (toast trigger)
	Expected        string
	ErrorsInSession int // running count of wrong/skip answers on this word in this session
}

// PendingTyping is the lookup payload stored when waiting for a typed reply.
type PendingTyping struct {
	UserID    int64
	ChatID    int64
	Source    string // "words" | "tasks"
	Payload   PendingPayload
	ExpiresAt time.Time
}

// PendingPayload is what we encode in the JSONB column.
type PendingPayload struct {
	// For source="words":
	SessionID int64 `json:"session_id,omitempty"`
	WordID    int64 `json:"word_id,omitempty"`
	Mode      Mode  `json:"mode,omitempty"`
	// For source="tasks":
	TaskID int64 `json:"task_id,omitempty"`
}
