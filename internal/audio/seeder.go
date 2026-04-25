// Package audio provides bulk-seeding of pronunciation data (transcription +
// audio_url) for words from the Free Dictionary API. Used by the standalone
// cmd/seed-audio tool and by the POST /admin/seed-audio HTTP endpoint.
package audio

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"makeMe/internal/dictionary"
)

// EventType describes a single line emitted while seeding.
type EventType string

const (
	EventStart   EventType = "start"   // beginning of a level
	EventWord    EventType = "word"    // per-word result
	EventDone    EventType = "done"    // overall summary at the end
	EventError   EventType = "error"   // unexpected error (level fetch failure, etc.)
)

// WordStatus describes the per-word outcome.
type WordStatus string

const (
	StatusUpdated     WordStatus = "updated"      // audio fetched and saved
	StatusAlreadyHave WordStatus = "already_have" // skipped: audio_url already populated
	StatusNotFound    WordStatus = "not_found"    // API returned no audio for this word
	StatusFailed      WordStatus = "failed"       // lookup error (network, 429, etc.)
)

// Event is one streaming output unit.
type Event struct {
	Type      EventType  `json:"event"`
	Level     string     `json:"level,omitempty"`
	Total     int        `json:"total,omitempty"`
	Word      string     `json:"word,omitempty"`
	Status    WordStatus `json:"status,omitempty"`
	Error     string     `json:"error,omitempty"`
	Scanned   int        `json:"scanned,omitempty"`
	Updated   int        `json:"updated,omitempty"`
	NotFound  int        `json:"not_found,omitempty"`
	Failed    int        `json:"failed,omitempty"`
	AlreadyOK int        `json:"already_have,omitempty"`
}

// Options controls a Seed run.
type Options struct {
	Levels  []string      // CEFR levels, e.g. ["A1","A2"]
	Delay   time.Duration // pause between API calls (>=100ms recommended to avoid 429)
}

// Seed scans the words table, fetches missing audio from the dictionary API,
// and persists results. It emits events on the returned channel and closes
// the channel when finished or when ctx is cancelled.
//
// Idempotent: rows that already have audio_url are skipped. Pre-existing
// transcription is preserved when the API doesn't return a new one.
func Seed(ctx context.Context, pool *pgxpool.Pool, dict *dictionary.Client, opts Options) <-chan Event {
	out := make(chan Event, 32)
	go func() {
		defer close(out)
		run(ctx, pool, dict, opts, out)
	}()
	return out
}

func run(ctx context.Context, pool *pgxpool.Pool, dict *dictionary.Client, opts Options, out chan<- Event) {
	var totals Event
	totals.Type = EventDone

	emit := func(ev Event) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- ev:
			return true
		}
	}

	for _, level := range opts.Levels {
		rows, err := pool.Query(ctx, `
			SELECT id, english, transcription, audio_url
			FROM words
			WHERE level = $1
			ORDER BY id
		`, level)
		if err != nil {
			if !emit(Event{Type: EventError, Level: level, Error: err.Error()}) {
				return
			}
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
				continue
			}
			words = append(words, w)
		}
		rows.Close()

		if !emit(Event{Type: EventStart, Level: level, Total: len(words)}) {
			return
		}

		for _, w := range words {
			totals.Scanned++
			if w.audioURL != "" {
				totals.AlreadyOK++
				if !emit(Event{Type: EventWord, Level: level, Word: w.english, Status: StatusAlreadyHave}) {
					return
				}
				continue
			}

			res, err := dict.Lookup(ctx, w.english)
			if err != nil {
				totals.Failed++
				if !emit(Event{Type: EventWord, Level: level, Word: w.english, Status: StatusFailed, Error: err.Error()}) {
					return
				}
				if !sleep(ctx, opts.Delay) {
					return
				}
				continue
			}
			if res.AudioURL == "" {
				totals.NotFound++
				if !emit(Event{Type: EventWord, Level: level, Word: w.english, Status: StatusNotFound}) {
					return
				}
				if !sleep(ctx, opts.Delay) {
					return
				}
				continue
			}

			transcription := res.Transcription
			if transcription == "" {
				transcription = w.transcription // preserve existing IPA
			}

			if _, err := pool.Exec(ctx,
				`UPDATE words SET transcription = $2, audio_url = $3 WHERE id = $1`,
				w.id, transcription, res.AudioURL,
			); err != nil {
				totals.Failed++
				if !emit(Event{Type: EventWord, Level: level, Word: w.english, Status: StatusFailed, Error: err.Error()}) {
					return
				}
				if !sleep(ctx, opts.Delay) {
					return
				}
				continue
			}

			totals.Updated++
			if !emit(Event{Type: EventWord, Level: level, Word: w.english, Status: StatusUpdated}) {
				return
			}
			if !sleep(ctx, opts.Delay) {
				return
			}
		}
	}

	emit(totals)
}

// sleep blocks for d but returns false if ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
