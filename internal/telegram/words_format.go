package telegram

import (
	"fmt"
	"math"
	"strings"

	"makeMe/internal/domain/words"
)

// formatLevelProgress renders a per-CEFR-level progress block for the /words
// intro screen and the /profile screen. Returns "" when the user has zero
// learned words at every level — in that case the whole block is suppressed
// so brand-new users see a cleaner screen until they earn their first row.
// Levels with Learned = 0 are also dropped individually, so only the levels
// the user has actually started appear. The block ends with a trailing newline
// so callers can append it directly without adding extra whitespace.
//
// Format:
//
//	📊 По уровням:
//	A1: 12/120 (10%)
//	A2: 5/180 (3%)
func formatLevelProgress(levels []words.LevelProgress) string {
	lines := make([]string, 0, len(levels))
	for _, lp := range levels {
		if lp.Total == 0 || lp.Learned == 0 {
			continue
		}
		pct := int(math.Round(100 * float64(lp.Learned) / float64(lp.Total)))
		lines = append(lines, fmt.Sprintf("%s: %d/%d (%d%%)", lp.Level, lp.Learned, lp.Total, pct))
	}
	if len(lines) == 0 {
		return ""
	}
	return "📊 По уровням:\n" + strings.Join(lines, "\n") + "\n"
}
