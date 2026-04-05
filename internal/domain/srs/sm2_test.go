package srs

import (
	"testing"
	"time"
)

func TestReview_FirstCorrect(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	card := ReviewCard{EaseFactor: 2.5}

	result := Review(card, QualityCorrectPerfect, now)

	if result.Repetitions != 1 {
		t.Errorf("repetitions = %d, want 1", result.Repetitions)
	}
	if result.IntervalDays != 1 {
		t.Errorf("interval = %d, want 1", result.IntervalDays)
	}
	if result.NextReviewDate != now.AddDate(0, 0, 1) {
		t.Errorf("next review = %v, want %v", result.NextReviewDate, now.AddDate(0, 0, 1))
	}
}

func TestReview_SecondCorrect(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	card := ReviewCard{EaseFactor: 2.5, Repetitions: 1, IntervalDays: 1}

	result := Review(card, QualityCorrectPerfect, now)

	if result.Repetitions != 2 {
		t.Errorf("repetitions = %d, want 2", result.Repetitions)
	}
	if result.IntervalDays != 6 {
		t.Errorf("interval = %d, want 6", result.IntervalDays)
	}
}

func TestReview_ThirdCorrect(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	card := ReviewCard{EaseFactor: 2.5, Repetitions: 2, IntervalDays: 6}

	result := Review(card, QualityCorrectPerfect, now)

	if result.Repetitions != 3 {
		t.Errorf("repetitions = %d, want 3", result.Repetitions)
	}
	// 6 * 2.5 = 15
	if result.IntervalDays != 15 {
		t.Errorf("interval = %d, want 15", result.IntervalDays)
	}
}

func TestReview_Incorrect_Resets(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	card := ReviewCard{EaseFactor: 2.5, Repetitions: 5, IntervalDays: 30}

	result := Review(card, QualityIncorrect, now)

	if result.Repetitions != 0 {
		t.Errorf("repetitions = %d, want 0", result.Repetitions)
	}
	if result.IntervalDays != 1 {
		t.Errorf("interval = %d, want 1", result.IntervalDays)
	}
}

func TestReview_EaseFactorMinimum(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	card := ReviewCard{EaseFactor: 1.3}

	// Quality 0 should decrease EF but not below 1.3
	result := Review(card, QualityBlackout, now)

	if result.EaseFactor < 1.3 {
		t.Errorf("ease factor = %f, should not go below 1.3", result.EaseFactor)
	}
}

func TestReview_EaseFactorIncreases(t *testing.T) {
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	card := ReviewCard{EaseFactor: 2.5}

	result := Review(card, QualityCorrectPerfect, now)

	if result.EaseFactor <= 2.5 {
		t.Errorf("ease factor = %f, should increase on perfect answer", result.EaseFactor)
	}
}

func TestQualityFromCorrect(t *testing.T) {
	if QualityFromCorrect(true) != QualityCorrectPerfect {
		t.Error("correct should return QualityCorrectPerfect")
	}
	if QualityFromCorrect(false) != QualityIncorrect {
		t.Error("incorrect should return QualityIncorrect")
	}
}
