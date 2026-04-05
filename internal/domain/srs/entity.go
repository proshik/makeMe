package srs

import (
	"time"

	"makeMe/internal/domain/user"
)

type Word struct {
	ID            int64
	English       string
	Russian       string
	Level         user.Level
	Example       string
	Transcription string
	AudioURL      string
}

type ReviewCard struct {
	ID             int64
	UserID         int64
	WordID         int64
	EaseFactor     float64
	IntervalDays   int
	Repetitions    int
	NextReviewDate time.Time
	UpdatedAt      time.Time
}
