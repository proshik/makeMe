package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"makeMe/internal/config"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
)

type WordEntry struct {
	English       string `json:"english"`
	Russian       string `json:"russian"`
	Level         string `json:"level"`
	Example       string `json:"example"`
	Transcription string `json:"transcription"`
}

type GrammarEntry struct {
	Level    string   `json:"level"`
	Category string   `json:"category"`
	Prompt   string   `json:"prompt"`
	Options  []string `json:"options"`
	Answer   string   `json:"answer"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: seed <path-to-file.json>")
	}

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("read file: %v", err)
	}

	queries := sqlc.New(pool)

	// Detect type by trying to unmarshal as grammar first (has "prompt" field),
	// then fall back to words.
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		log.Fatalf("parse json: %v", err)
	}

	var probe struct {
		Prompt *string `json:"prompt"`
	}
	if err := json.Unmarshal(raw[0], &probe); err == nil && probe.Prompt != nil {
		seedGrammar(ctx, queries, data)
	} else {
		seedWords(ctx, queries, data)
	}
}

func seedWords(ctx context.Context, queries *sqlc.Queries, data []byte) {
	var words []WordEntry
	if err := json.Unmarshal(data, &words); err != nil {
		log.Fatalf("parse words json: %v", err)
	}

	inserted := 0
	for _, w := range words {
		err := queries.UpsertWord(ctx, sqlc.UpsertWordParams{
			English:       w.English,
			Russian:       w.Russian,
			Level:         w.Level,
			Example:       w.Example,
			Transcription: w.Transcription,
		})
		if err != nil {
			log.Printf("failed to upsert word %q: %v", w.English, err)
			continue
		}
		inserted++
	}

	fmt.Printf("Done: %d/%d words upserted\n", inserted, len(words))
}

func seedGrammar(ctx context.Context, queries *sqlc.Queries, data []byte) {
	var questions []GrammarEntry
	if err := json.Unmarshal(data, &questions); err != nil {
		log.Fatalf("parse grammar json: %v", err)
	}

	inserted := 0
	for _, q := range questions {
		optionsJSON, err := json.Marshal(q.Options)
		if err != nil {
			log.Printf("failed to marshal options for %q: %v", q.Prompt, err)
			continue
		}
		err = queries.UpsertGrammarQuestion(ctx, sqlc.UpsertGrammarQuestionParams{
			Level:    q.Level,
			Category: q.Category,
			Prompt:   q.Prompt,
			Options:  optionsJSON,
			Answer:   q.Answer,
		})
		if err != nil {
			log.Printf("failed to upsert grammar %q: %v", q.Prompt, err)
			continue
		}
		inserted++
	}

	fmt.Printf("Done: %d/%d grammar questions upserted\n", inserted, len(questions))
}
