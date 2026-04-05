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

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: seed <path-to-words.json>")
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

	var words []WordEntry
	if err := json.Unmarshal(data, &words); err != nil {
		log.Fatalf("parse json: %v", err)
	}

	queries := sqlc.New(pool)

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
			log.Printf("failed to upsert %q: %v", w.English, err)
			continue
		}
		inserted++
	}

	fmt.Printf("Done: %d/%d words upserted\n", inserted, len(words))
}
