// Command seed-audio bulk-fetches pronunciation data from the Free Dictionary
// API for words at the requested CEFR levels and persists transcription +
// audio_url to the words table.
//
// Idempotent: words that already have an audio_url are skipped, so re-running
// is safe. Words for which the API returns no audio are left untouched (they
// can be retried on the next run, or via the in-app /tasks audio button).
//
// Usage:
//
//	go run ./cmd/seed-audio -levels=A1,A2
//	go run ./cmd/seed-audio -levels=B1 -delay-ms=200
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"makeMe/internal/config"
	"makeMe/internal/dictionary"
)

func main() {
	levelsFlag := flag.String("levels", "A1,A2", "comma-separated CEFR levels to seed")
	delayMs := flag.Int("delay-ms", 150, "delay between API calls in milliseconds (be nice to free API)")
	flag.Parse()

	levels := splitAndTrim(*levelsFlag)
	if len(levels) == 0 {
		slog.Error("no levels specified")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to db", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	dict := dictionary.NewClient()

	var (
		scanned, alreadyHave, updated, audioFound, notFound, failed int
	)

	for _, level := range levels {
		rows, err := pool.Query(ctx, `
			SELECT id, english, transcription, audio_url
			FROM words
			WHERE level = $1
			ORDER BY id
		`, level)
		if err != nil {
			slog.Error("query level", "level", level, "error", err)
			continue
		}

		type wrow struct {
			id            int64
			english       string
			transcription string
			audioURL      string
		}
		var words []wrow
		for rows.Next() {
			var w wrow
			if err := rows.Scan(&w.id, &w.english, &w.transcription, &w.audioURL); err != nil {
				slog.Error("scan row", "error", err)
				continue
			}
			words = append(words, w)
		}
		rows.Close()

		slog.Info("processing level", "level", level, "total", len(words))

		for i, w := range words {
			scanned++
			if w.audioURL != "" {
				alreadyHave++
				continue
			}

			res, err := dict.Lookup(ctx, w.english)
			if err != nil {
				slog.Warn("lookup failed", "word", w.english, "error", err)
				failed++
				time.Sleep(time.Duration(*delayMs) * time.Millisecond)
				continue
			}
			if res.AudioURL == "" {
				notFound++
				slog.Debug("no audio in API response", "word", w.english, "ipa_returned", res.Transcription != "")
				time.Sleep(time.Duration(*delayMs) * time.Millisecond)
				continue
			}

			// Don't overwrite an existing transcription with an empty one from this lookup.
			transcription := res.Transcription
			if transcription == "" {
				transcription = w.transcription
			}

			if _, err := pool.Exec(ctx,
				`UPDATE words SET transcription = $2, audio_url = $3 WHERE id = $1`,
				w.id, transcription, res.AudioURL,
			); err != nil {
				slog.Error("update failed", "word", w.english, "error", err)
				failed++
				time.Sleep(time.Duration(*delayMs) * time.Millisecond)
				continue
			}
			updated++
			audioFound++

			if (i+1)%25 == 0 {
				slog.Info("progress", "level", level, "done", i+1, "of", len(words))
			}

			time.Sleep(time.Duration(*delayMs) * time.Millisecond)
		}
	}

	fmt.Printf("\nDone. scanned=%d already_had_audio=%d updated=%d audio_found=%d not_found=%d failed=%d\n",
		scanned, alreadyHave, updated, audioFound, notFound, failed)
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
