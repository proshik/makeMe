package server

import (
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"makeMe/internal/dictionary"
	"makeMe/internal/telegram"
	"makeMe/internal/web"
)

// AdminConfig groups dependencies needed by /admin/* endpoints. AdminToken
// being empty disables those endpoints (they respond 503), so the server can
// still start without admin tooling configured.
type AdminConfig struct {
	Pool       *pgxpool.Pool
	DictClient *dictionary.Client
	AdminToken string
}

// New builds the HTTP router. webSrv may be nil — the web UI is optional and
// the bot works without it (see Config.WebEnabled). webhookSecret is the
// secret_token registered with setWebhook; with it empty /webhook answers 503.
func New(tgBot *telegram.Bot, webhookSecret string, admin AdminConfig, webSrv *web.Server) *chi.Mux {
	r := chi.NewRouter()
	useCommonMiddleware(r)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Post("/webhook", webhookAuth(webhookSecret, tgBot.WebhookHandler()))

	deps := newAdminDeps(admin)
	r.Route("/admin", func(ar chi.Router) {
		ar.Post("/seed-audio", adminAuth(deps.token, deps.handleSeedAudio))
	})

	if webSrv != nil {
		webSrv.Mount(r)
	}

	return r
}

// useCommonMiddleware installs the middleware every route runs behind. The
// seed-audio tests mount the handler through it too: streaming depends on the
// ResponseWriter chain (deadlines and flushes reach the connection only
// through Unwrap), so testing the bare handler would not cover production.
func useCommonMiddleware(r chi.Router) {
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
}

// webhookSecretHeader carries the secret_token given to setWebhook on every
// update Telegram delivers.
const webhookSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// webhookAuth admits only requests carrying the webhook secret. go-telegram/bot
// has a check of its own (WithWebhookSecretToken), deliberately not used: it
// compares with != and, on a mismatch, returns without writing a status, so a
// forged request gets 200. Here the comparison is constant-time and a mismatch
// is a 401 before the body is read.
//
// An empty secret closes the endpoint (503), the same way an empty ADMIN_TOKEN
// closes /admin/*. Config allows that only without WEBHOOK_URL, when Telegram
// has no reason to call here at all.
func webhookAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			http.Error(w, "webhook disabled (set TELEGRAM_WEBHOOK_SECRET)", http.StatusServiceUnavailable)
			return
		}
		got := r.Header.Get(webhookSecretHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
