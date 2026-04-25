package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"makeMe/internal/dictionary"
	"makeMe/internal/telegram"
)

// AdminConfig groups dependencies needed by /admin/* endpoints. AdminToken
// being empty disables those endpoints (they respond 503), so the server can
// still start without admin tooling configured.
type AdminConfig struct {
	Pool       *pgxpool.Pool
	DictClient *dictionary.Client
	AdminToken string
}

func New(tgBot *telegram.Bot, admin AdminConfig) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Post("/webhook", tgBot.WebhookHandler())

	deps := &adminDeps{
		pool:       admin.Pool,
		dictClient: admin.DictClient,
		token:      admin.AdminToken,
	}
	r.Route("/admin", func(ar chi.Router) {
		ar.Post("/seed-audio", adminAuth(deps.token, deps.handleSeedAudio))
	})

	return r
}
