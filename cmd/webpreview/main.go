// Command webpreview boots the real web adapter against a local database and a
// stub OIDC provider, so the UI can be inspected in a browser without Telegram
// credentials.
//
// DEV ONLY. It exposes /dev/session, which mints a session for a fixed user
// without going through OAuth at all. The Dockerfile builds ./cmd/makeme only,
// so this binary is never deployed — keep it that way.
//
//	docker run -d --name makeme-preview-db \
//	  -e POSTGRES_USER=makeme -e POSTGRES_PASSWORD=makeme -e POSTGRES_DB=makeme \
//	  -p 55433:5432 postgres:17
//	go run ./cmd/webpreview
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
	"makeMe/internal/web"
)

const previewAddr = ":8900"

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("PREVIEW_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://makeme:makeme@localhost:55433/makeme?sslmode=disable"
	}

	if err := migrate(dbURL); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)
	userSvc := user.NewService(postgres.NewUserRepository(q))
	groupSvc := group.NewService(postgres.NewGroupRepository(q))
	srsRepo := postgres.NewSRSRepository(q)
	grammarRepo := postgres.NewGrammarRepository(q)
	taskSvc := task.NewService(
		postgres.NewTaskRepository(q), srsRepo, 5,
		task.NewVocabGenerator(srsRepo), task.NewGrammarGenerator(grammarRepo),
	)
	scoringSvc := scoring.NewService(postgres.NewScoringRepository(q))
	assessSvc := assessment.NewService(postgres.NewAssessmentRepository(q), userSvc, srsRepo, grammarRepo)

	stub := stubOIDC()
	defer stub.Close()

	jwtSvc, err := web.NewJWTService(
		base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), 24)
	if err != nil {
		log.Fatal(err)
	}

	srv, err := web.New(web.Deps{
		UserSvc: userSvc, GroupSvc: groupSvc, TaskSvc: taskSvc,
		ScoringSvc: scoringSvc, SRSRepo: srsRepo, AssessSvc: assessSvc,
	}, jwtSvc, web.NewOIDCClient(stub.URL, "client-1", "secret",
		"http://localhost"+previewAddr+"/auth/telegram/callback"), false)
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Get("/dev/session", func(w http.ResponseWriter, req *http.Request) {
		u, err := userSvc.Register(req.Context(), 777, "alice", "Алиса")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !u.Onboarded {
			if err := userSvc.UpdateLevel(req.Context(), u.ID, user.B2); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		token, err := jwtSvc.Create(u.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: token, Path: "/", HttpOnly: true})
		http.Redirect(w, req, "/", http.StatusSeeOther)
	})
	srv.Mount(r)

	fmt.Println("preview on http://localhost" + previewAddr + "/login  (shortcut: /dev/session)")
	log.Fatal(http.ListenAndServe(previewAddr, r))
}

func migrate(dbURL string) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return goose.Up(db, "migrations")
}

// stubOIDC emulates the consent screen and token endpoint, so the whole login
// flow is clickable without real credentials.
func stubOIDC() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth" {
			http.Redirect(w, r, fmt.Sprintf(
				"http://localhost%s/auth/telegram/callback?code=stub&state=%s",
				previewAddr, r.URL.Query().Get("state")), http.StatusSeeOther)
			return
		}
		enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
		claims, _ := json.Marshal(map[string]any{
			"id": float64(777), "name": "Алиса", "preferred_username": "alice",
			"iss": "https://oauth.telegram.org", "aud": "client-1",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token": enc(`{"alg":"none"}`) + "." + enc(string(claims)) + ".sig",
		})
	}))
}
