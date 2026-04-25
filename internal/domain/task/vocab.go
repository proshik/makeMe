package task

import (
	"context"
	"fmt"
	"html"
	"math/rand/v2"
	"strings"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/user"
)

// VocabGenerator generates vocabulary tasks using SRS data.
type VocabGenerator struct {
	srsRepo srs.Repository
}

func NewVocabGenerator(srsRepo srs.Repository) *VocabGenerator {
	return &VocabGenerator{srsRepo: srsRepo}
}

func (g *VocabGenerator) Type() TaskType { return TypeVocab }

func (g *VocabGenerator) Generate(ctx context.Context, userID int64, level user.Level, count int) ([]Task, error) {
	// Get due review cards first
	dueCards, err := g.srsRepo.GetDueCards(ctx, userID, count)
	if err != nil {
		return nil, fmt.Errorf("get due cards: %w", err)
	}

	tasks := make([]Task, 0, count)

	// Add review tasks from due cards
	for _, card := range dueCards {
		if len(tasks) >= count {
			break
		}
		word, err := g.srsRepo.GetWordByID(ctx, card.WordID)
		if err != nil {
			continue
		}
		t, err := g.wordToTask(ctx, word, level)
		if err != nil {
			continue
		}
		tasks = append(tasks, t)
	}

	// Fill remaining slots with new words
	remaining := count - len(tasks)
	if remaining > 0 {
		newWords, err := g.srsRepo.GetNewWordsForUser(ctx, userID, level, remaining)
		if err != nil {
			return nil, fmt.Errorf("get new words: %w", err)
		}
		for _, word := range newWords {
			t, err := g.wordToTask(ctx, &word, level)
			if err != nil {
				continue
			}
			tasks = append(tasks, t)
		}
	}

	return tasks, nil
}

func (g *VocabGenerator) wordToTask(ctx context.Context, word *srs.Word, level user.Level) (Task, error) {
	// Get distractor words at the same level for wrong options
	distractors, err := g.srsRepo.GetWordsByLevel(ctx, word.Level, 10)
	if err != nil {
		return Task{}, fmt.Errorf("get distractors: %w", err)
	}

	// Randomly pick a subtype; fill_blank requires an example sentence.
	subtypes := []string{
		VocabSubtypeEnRu,
		VocabSubtypeRuEn,
		VocabSubtypeFillBlank,
		VocabSubtypeTypingEnRu,
		VocabSubtypeTypingRuEn,
	}
	subtype := subtypes[rand.IntN(len(subtypes))]

	var t Task
	switch subtype {
	case VocabSubtypeRuEn:
		t = buildVocabRuEn(word, distractors)
	case VocabSubtypeFillBlank:
		t = buildVocabFillBlank(word, distractors)
	case VocabSubtypeTypingEnRu:
		t = buildVocabTypingEnRu(word)
	case VocabSubtypeTypingRuEn:
		t = buildVocabTypingRuEn(word)
	default:
		t = buildVocabEnRu(word, distractors)
	}

	t.Metadata["word_id"] = word.ID
	t.Metadata["audio_url"] = word.AudioURL
	return t, nil
}

// buildVocabEnRu builds an EN→RU task: English prompt, Russian options.
func buildVocabEnRu(word *srs.Word, distractors []srs.Word) Task {
	prompt := html.EscapeString(word.English)
	if word.Transcription != "" {
		prompt = fmt.Sprintf("%s  %s", prompt, html.EscapeString(word.Transcription))
	}
	if word.Example != "" {
		prompt = fmt.Sprintf("%s\n\n<i>%s</i>", prompt, html.EscapeString(word.Example))
	}

	options := buildOptions(word.Russian, distractors, func(d srs.Word) string { return d.Russian })

	return Task{
		Type:    TypeVocab,
		Level:   word.Level,
		Prompt:  prompt,
		Options: options,
		Answer:  word.Russian,
		Metadata: map[string]any{
			"subtype": VocabSubtypeEnRu,
		},
	}
}

// buildVocabRuEn builds an RU→EN task: Russian prompt, English options.
func buildVocabRuEn(word *srs.Word, distractors []srs.Word) Task {
	options := buildOptions(word.English, distractors, func(d srs.Word) string { return d.English })

	return Task{
		Type:    TypeVocab,
		Level:   word.Level,
		Prompt:  html.EscapeString(word.Russian),
		Options: options,
		Answer:  word.English,
		Metadata: map[string]any{
			"subtype": VocabSubtypeRuEn,
		},
	}
}

// buildVocabFillBlank builds a fill-in-the-blank task using the example sentence.
// Falls back to en_ru if the word is not present in the example.
func buildVocabFillBlank(word *srs.Word, distractors []srs.Word) Task {
	blanked, ok := blankWord(word.Example, word.English)
	if !ok {
		return buildVocabEnRu(word, distractors)
	}

	options := buildOptions(word.English, distractors, func(d srs.Word) string { return d.English })

	return Task{
		Type:    TypeVocab,
		Level:   word.Level,
		Prompt:  html.EscapeString(blanked),
		Options: options,
		Answer:  word.English,
		Metadata: map[string]any{
			"subtype": VocabSubtypeFillBlank,
		},
	}
}

// buildVocabTypingEnRu builds a typing task: prompt is the English word
// (with optional transcription/example), the user must type the Russian.
// Options is empty — typing uses textmatch validation, not multiple choice.
func buildVocabTypingEnRu(word *srs.Word) Task {
	prompt := html.EscapeString(word.English)
	if word.Transcription != "" {
		prompt = fmt.Sprintf("%s  %s", prompt, html.EscapeString(word.Transcription))
	}
	if word.Example != "" {
		prompt = fmt.Sprintf("%s\n\n<i>%s</i>", prompt, html.EscapeString(word.Example))
	}
	return Task{
		Type:    TypeVocab,
		Level:   word.Level,
		Prompt:  prompt,
		Options: []string{},
		Answer:  word.Russian,
		Metadata: map[string]any{
			"subtype": VocabSubtypeTypingEnRu,
		},
	}
}

// buildVocabTypingRuEn is the mirror: Russian prompt, user types English.
func buildVocabTypingRuEn(word *srs.Word) Task {
	return Task{
		Type:    TypeVocab,
		Level:   word.Level,
		Prompt:  html.EscapeString(word.Russian),
		Options: []string{},
		Answer:  word.English,
		Metadata: map[string]any{
			"subtype": VocabSubtypeTypingRuEn,
		},
	}
}

// blankWord replaces the first occurrence of word (case-insensitive) in sentence
// with "___". Returns the modified sentence and true if found.
func blankWord(sentence, word string) (string, bool) {
	if sentence == "" || word == "" {
		return "", false
	}
	lower := strings.ToLower(sentence)
	target := strings.ToLower(word)
	idx := strings.Index(lower, target)
	if idx < 0 {
		return "", false
	}
	return sentence[:idx] + "___" + sentence[idx+len(word):], true
}

// buildOptions assembles 4 shuffled answer choices: the correct answer plus up to 3
// distractors drawn from distractor words, excluding any duplicate of the correct answer.
func buildOptions(correct string, distractors []srs.Word, pick func(srs.Word) string) []string {
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
	return options
}
