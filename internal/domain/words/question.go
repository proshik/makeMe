package words

import (
	"math/rand/v2"

	"makeMe/internal/domain/srs"
)

// ExpectedAnswer is the correct response for a (word, mode) pair: the Russian
// side when drilling EN→RU, the English side when drilling RU→EN.
func ExpectedAnswer(word *srs.Word, mode Mode) string {
	return expectedAnswer(word, mode)
}

// BuildMCOptions assembles up to 4 shuffled options — the correct answer plus
// distractors drawn from the same CEFR level — and returns the option list with
// the index of the correct one.
//
// This lives in the domain rather than in an adapter because it is the shape of
// the question, not the shape of the presentation: both the bot and the web UI
// must draw from the same pool in the same way, or the two channels would be
// drilling the user differently.
func BuildMCOptions(correct string, distractors []srs.Word, mode Mode) ([]string, int) {
	pick := pickRussian
	if mode == ModeRuEnMC || mode == ModeRuEnTyping {
		pick = pickEnglish
	}

	options := []string{correct}
	for _, d := range distractors {
		v := pick(d)
		if v == correct {
			continue
		}
		options = append(options, v)
		if len(options) >= 4 {
			break
		}
	}

	rand.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})

	correctIdx := 0
	for i, o := range options {
		if o == correct {
			correctIdx = i
			break
		}
	}
	return options, correctIdx
}

func pickRussian(w srs.Word) string { return w.Russian }
func pickEnglish(w srs.Word) string { return w.English }
