package server

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"makeMe/internal/audio"
	"makeMe/internal/dictionary"
)

// adminDeps groups the dependencies an admin handler needs. Kept private to
// the package; constructed inside server.New from the public AdminConfig.
type adminDeps struct {
	pool       *pgxpool.Pool
	dictClient *dictionary.Client
	token      string // empty => admin endpoints respond 503
}

// adminAuth wraps a handler with constant-time bearer-token check.
// If no token is configured (cfg.AdminToken == ""), the endpoint returns 503
// to avoid accidental open access from a forgotten env var.
func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.Error(w, "admin endpoints disabled (set ADMIN_TOKEN)", http.StatusServiceUnavailable)
			return
		}
		got := bearerToken(r)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if !strings.HasPrefix(h, p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}

// seedAudioRequest is the JSON body accepted by POST /admin/seed-audio.
type seedAudioRequest struct {
	Levels  []string `json:"levels"`
	DelayMs int      `json:"delay_ms"`
}

// handleSeedAudio streams ndjson progress while bulk-fetching pronunciation.
// The response is chunked; clients should read line-by-line ("curl -N").
func (d *adminDeps) handleSeedAudio(w http.ResponseWriter, r *http.Request) {
	if d.pool == nil || d.dictClient == nil {
		http.Error(w, "audio seeding not wired", http.StatusServiceUnavailable)
		return
	}

	var req seedAudioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(req.Levels) == 0 {
		http.Error(w, `"levels" must be non-empty`, http.StatusBadRequest)
		return
	}
	delay := time.Duration(req.DelayMs) * time.Millisecond
	if delay <= 0 {
		delay = 500 * time.Millisecond
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	slog.Info("admin seed-audio started", "levels", req.Levels, "delay_ms", req.DelayMs, "remote", r.RemoteAddr)

	enc := json.NewEncoder(w)
	events := audio.Seed(r.Context(), d.pool, d.dictClient, audio.Options{
		Levels: req.Levels,
		Delay:  delay,
	})
	for ev := range events {
		if err := enc.Encode(ev); err != nil {
			slog.Warn("admin seed-audio encode failed (client disconnect?)", "error", err)
			return
		}
		flusher.Flush()
	}

	slog.Info("admin seed-audio completed", "levels", req.Levels)
}
