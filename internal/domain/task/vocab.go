package task

import (
	"context"
	"fmt"
	"math/rand/v2"

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

	options := []string{word.Russian}
	for _, d := range distractors {
		if d.ID == word.ID || d.Russian == word.Russian {
			continue
		}
		options = append(options, d.Russian)
		if len(options) >= 4 {
			break
		}
	}

	// Shuffle options
	rand.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})

	prompt := word.English
	if word.Transcription != "" {
		prompt = fmt.Sprintf("%s  %s", word.English, word.Transcription)
	}
	if word.Example != "" {
		prompt = fmt.Sprintf("%s\n\n_%s_", prompt, word.Example)
	}

	return Task{
		Type:    TypeVocab,
		Level:   word.Level,
		Prompt:  prompt,
		Options: options,
		Answer:  word.Russian,
		Metadata: map[string]any{
			"word_id":   word.ID,
			"audio_url": word.AudioURL,
		},
	}, nil
}
