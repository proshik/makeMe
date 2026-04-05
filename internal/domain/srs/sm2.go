package srs

import "time"

// Quality grades for SM-2 algorithm (0–5).
const (
	QualityBlackout       = 0 // Complete blackout
	QualityIncorrect      = 1 // Incorrect, but upon seeing the answer, remembered
	QualityIncorrectEasy  = 2 // Incorrect, but answer seemed easy to recall
	QualityCorrectHard    = 3 // Correct with serious difficulty
	QualityCorrectHesit   = 4 // Correct after hesitation
	QualityCorrectPerfect = 5 // Perfect response
)

// Review applies the SM-2 algorithm to a card given a quality grade (0–5).
// Returns an updated copy of the card.
func Review(card ReviewCard, quality int, now time.Time) ReviewCard {
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	c := card

	if quality >= 3 {
		// Correct response
		switch c.Repetitions {
		case 0:
			c.IntervalDays = 1
		case 1:
			c.IntervalDays = 6
		default:
			c.IntervalDays = int(float64(c.IntervalDays) * c.EaseFactor)
			if c.IntervalDays < 1 {
				c.IntervalDays = 1
			}
		}
		c.Repetitions++
	} else {
		// Incorrect response — reset
		c.Repetitions = 0
		c.IntervalDays = 1
	}

	// Update ease factor: EF' = EF + (0.1 - (5-q) * (0.08 + (5-q)*0.02))
	delta := 0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02)
	c.EaseFactor += delta
	if c.EaseFactor < 1.3 {
		c.EaseFactor = 1.3
	}

	c.NextReviewDate = now.AddDate(0, 0, c.IntervalDays)
	c.UpdatedAt = now

	return c
}

// QualityFromCorrect converts a boolean answer into an SM-2 quality grade.
func QualityFromCorrect(correct bool) int {
	if correct {
		return QualityCorrectPerfect
	}
	return QualityIncorrect
}
