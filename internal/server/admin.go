package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"makeMe/internal/audio"
)

// adminDeps groups the dependencies an admin handler needs. Kept private to
// the package; constructed inside server.New from the public AdminConfig.
type adminDeps struct {
	// seed produces the seeding event stream. server.New binds it to
	// audio.Seed when a pool and a dictionary client are configured; nil
	// means audio seeding is not wired. Tests substitute their own stream.
	seed  func(ctx context.Context, opts audio.Options) <-chan audio.Event
	token string // empty => admin endpoints respond 503
	// lineTimeout bounds each streamed write; zero means seedAudioLineTimeout.
	lineTimeout time.Duration
}

// seedAudioLineTimeout bounds a single ndjson line of POST /admin/seed-audio.
// It replaces the server's WriteTimeout for that response, which bounds the
// whole response and would cut a run that lasts minutes. The gap between lines
// does not count: the deadline is re-armed right before every write.
const seedAudioLineTimeout = 30 * time.Second

// newAdminDeps wires the admin handlers from the public AdminConfig.
func newAdminDeps(cfg AdminConfig) *adminDeps {
	d := &adminDeps{token: cfg.AdminToken}
	if cfg.Pool != nil && cfg.DictClient != nil {
		d.seed = func(ctx context.Context, opts audio.Options) <-chan audio.Event {
			return audio.Seed(ctx, cfg.Pool, cfg.DictClient, opts)
		}
	}
	return d
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
	if d.seed == nil {
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

	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// The server's WriteTimeout bounds the whole response, and a run lasts
	// minutes. Replace it with a deadline per line: a long run streams to the
	// end, and a client that stops reading still releases the run.
	lineTimeout := d.lineTimeout
	if lineTimeout <= 0 {
		lineTimeout = seedAudioLineTimeout
	}
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Now().Add(lineTimeout)); err != nil {
		slog.Error("admin seed-audio: cannot set write deadline", "error", err)
		http.Error(w, "cannot set write deadline for streaming", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	slog.Info("admin seed-audio started", "levels", req.Levels, "delay_ms", req.DelayMs, "remote", r.RemoteAddr)

	enc := json.NewEncoder(w)
	events := d.seed(r.Context(), audio.Options{
		Levels: req.Levels,
		Delay:  delay,
	})
	for ev := range events {
		if err := rc.SetWriteDeadline(time.Now().Add(lineTimeout)); err != nil {
			slog.Warn("admin seed-audio: cannot set write deadline", "error", err)
			return
		}
		if err := enc.Encode(ev); err != nil {
			slog.Warn("admin seed-audio encode failed (client disconnect?)", "error", err)
			return
		}
		// http.Flusher swallows write errors; the controller reports them, so
		// a write that cannot finish within lineTimeout (the client stopped
		// reading and the socket buffers are full) ends the handler and, with
		// it, the run.
		if err := rc.Flush(); err != nil {
			slog.Warn("admin seed-audio flush failed (client stalled or gone?)", "error", err)
			return
		}
	}

	slog.Info("admin seed-audio completed", "levels", req.Levels)
}
