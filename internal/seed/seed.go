// Package seed loads the JSON content files into the database.
//
// It lives in its own package so the application entrypoint and the dev
// preview tool share one implementation — per the project rule that bulk
// operational logic is never duplicated between a cmd/ tool and its caller.
//
// Content (words, grammar questions, grammar category theory) is seeded on
// every startup and is deliberately NOT carried in migrations: migrations are
// for schema only.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"makeMe/internal/storage/sqlc"
)

type wordEntry struct {
	English       string `json:"english"`
	Russian       string `json:"russian"`
	Level         string `json:"level"`
	Example       string `json:"example"`
	Transcription string `json:"transcription"`
}

type grammarCategoryEntry struct {
	Name        string `json:"name"`
	Explanation string `json:"explanation"`
}

type grammarQuestionEntry struct {
	Level       string   `json:"level"`
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory"`
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Answer      string   `json:"answer"`
}

// Words upserts the word list. audio_url is never written here — it is fetched
// lazily from the dictionary API, and overwriting it from JSON would discard
// pronunciations already collected.
func Words(ctx context.Context, q *sqlc.Queries, file string) error {
	var entries []wordEntry
	if err := readJSON(file, &entries); err != nil {
		return err
	}
	for _, w := range entries {
		if err := q.UpsertWord(ctx, sqlc.UpsertWordParams{
			English:       w.English,
			Russian:       w.Russian,
			Level:         w.Level,
			Example:       w.Example,
			Transcription: w.Transcription,
		}); err != nil {
			slog.Warn("seed: upsert word failed", "word", w.English, "error", err)
		}
	}
	slog.Info("seed: words", "count", len(entries))
	return nil
}

func GrammarCategories(ctx context.Context, q *sqlc.Queries, file string) error {
	var entries []grammarCategoryEntry
	if err := readJSON(file, &entries); err != nil {
		return err
	}
	for _, e := range entries {
		if err := q.UpsertGrammarCategory(ctx, sqlc.UpsertGrammarCategoryParams{
			Name:        e.Name,
			Explanation: e.Explanation,
		}); err != nil {
			slog.Warn("seed: upsert grammar category failed", "name", e.Name, "error", err)
		}
	}
	slog.Info("seed: grammar categories", "count", len(entries))
	return nil
}

func GrammarQuestions(ctx context.Context, q *sqlc.Queries, file string) error {
	var entries []grammarQuestionEntry
	if err := readJSON(file, &entries); err != nil {
		return err
	}
	for _, e := range entries {
		options, err := json.Marshal(e.Options)
		if err != nil {
			slog.Warn("seed: marshal options failed", "prompt", e.Prompt, "error", err)
			continue
		}
		subcategory := e.Subcategory
		if subcategory == "" {
			// Unknown or absent values fall back to the catch-all bucket.
			subcategory = "general"
		}
		if err := q.UpsertGrammarQuestion(ctx, sqlc.UpsertGrammarQuestionParams{
			Level:       e.Level,
			Category:    e.Category,
			Subcategory: subcategory,
			Prompt:      e.Prompt,
			Options:     options,
			Answer:      e.Answer,
		}); err != nil {
			slog.Warn("seed: upsert grammar question failed", "prompt", e.Prompt, "error", err)
		}
	}
	slog.Info("seed: grammar questions", "count", len(entries))
	return nil
}

// All runs every seeder against its own short-lived pool. A missing or
// malformed file is logged and skipped — startup continues, because stale
// content is better than a service that refuses to boot.
func All(ctx context.Context, dbURL, wordsFile, categoriesFile, questionsFile string) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Warn("seed skipped: cannot connect", "error", err)
		return
	}
	defer pool.Close()

	q := sqlc.New(pool)
	if err := Words(ctx, q, wordsFile); err != nil {
		slog.Warn("word seed skipped", "error", err)
	}
	if err := GrammarCategories(ctx, q, categoriesFile); err != nil {
		slog.Warn("grammar categories seed skipped", "error", err)
	}
	if err := GrammarQuestions(ctx, q, questionsFile); err != nil {
		slog.Warn("grammar questions seed skipped", "error", err)
	}
}

func readJSON(file string, out any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}
	return nil
}
