package server

import (
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
// the bot works without it (see Config.WebEnabled).
func New(tgBot *telegram.Bot, admin AdminConfig, webSrv *web.Server) *chi.Mux {
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

	if webSrv != nil {
		webSrv.Mount(r)
	}

	return r
}
