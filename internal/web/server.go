// Package web is the browser-facing adapter. Like internal/telegram it is a
// thin layer: it parses HTTP, calls the same domain services the bot calls, and
// renders the result. No business logic lives here — that is what keeps the two
// channels in sync without a synchronisation mechanism.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
)

//go:embed templates/*.html templates/fragments/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Deps are the collaborators the web adapter needs. Every one of them is the
// same instance the Telegram bot uses — that shared state is what makes a
// session started in the browser continue in the bot.
type Deps struct {
	UserSvc     *user.Service
	GroupSvc    *group.Service
	AssessSvc   *assessment.Service
	WordsSvc    *words.Service
	GrammarRepo task.GrammarRepository
	TaskSvc     *task.Service
	ScoringSvc  *scoring.Service
	SRSRepo     srs.Repository
}

type Server struct {
	userSvc     *user.Service
	groupSvc    *group.Service
	assessSvc   *assessment.Service
	wordsSvc    *words.Service
	grammarRepo task.GrammarRepository
	taskSvc     *task.Service
	scoringSvc  *scoring.Service
	srsRepo     srs.Repository

	jwt          *JWTService
	oidc         *OIDCClient
	tpl          *template.Template
	cookieSecure bool
}

func New(deps Deps, jwtSvc *JWTService, oidc *OIDCClient, cookieSecure bool) (*Server, error) {
	version, err := staticVersion()
	if err != nil {
		return nil, err
	}

	tpl, err := template.New("").Funcs(templateFuncs(version)).ParseFS(templateFS,
		"templates/*.html", "templates/fragments/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		userSvc:      deps.UserSvc,
		groupSvc:     deps.GroupSvc,
		assessSvc:    deps.AssessSvc,
		wordsSvc:     deps.WordsSvc,
		grammarRepo:  deps.GrammarRepo,
		taskSvc:      deps.TaskSvc,
		scoringSvc:   deps.ScoringSvc,
		srsRepo:      deps.SRSRepo,
		jwt:          jwtSvc,
		oidc:         oidc,
		tpl:          tpl,
		cookieSecure: cookieSecure,
	}, nil
}

// Mount attaches the web UI to an existing router. Routes are registered on the
// caller's mux so the web UI, the Telegram webhook and /admin all share one
// server and one origin.
func (s *Server) Mount(r chi.Router) {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: static assets missing from embed: " + err.Error())
	}

	r.Group(func(pub chi.Router) {
		pub.Use(securityHeaders)

		pub.Handle("/static/*", http.StripPrefix("/static/",
			cacheStatic(http.FileServer(http.FS(static)))))

		pub.Get("/login", s.handleLogin)
		pub.Get("/auth/telegram", s.handleAuthStart)
		pub.Get("/auth/telegram/callback", s.handleAuthCallback)
		pub.Get("/logout", s.handleLogout)
	})

	// Своя 404 вместо голого chi-ответа: у той нет ни навигации, ни выхода,
	// и она игнорирует тему.
	r.NotFound(securityHeaders(http.HandlerFunc(s.handleNotFound)).ServeHTTP)

	r.Group(func(priv chi.Router) {
		priv.Use(securityHeaders)
		priv.Use(s.requireAuth)

		priv.Get("/", s.handleHome)

		priv.Get("/tasks", s.handleTasks)
		priv.Get("/tasks/next", s.handleTaskNext)
		priv.Post("/tasks/{id}/answer", s.handleTaskAnswer)

		priv.Get("/assess", s.handleAssess)
		priv.Post("/assess/answer", s.handleAssessAnswer)

		priv.Get("/words", s.handleWords)
		priv.Get("/words/next", s.handleWordsNext)
		priv.Post("/words/{id}/answer", s.handleWordsAnswer)
		priv.Post("/words/{id}/know", s.handleWordsKnow)
		priv.Post("/words/{id}/defer", s.handleWordsDefer)

		priv.Get("/learn", s.handleLearn)
		priv.Get("/learn/topic", s.handleLearnTopic)
		priv.Get("/learn/practice", s.handleLearnPractice)
		priv.Get("/learn/next", s.handleLearnNext)
		priv.Post("/learn/answer", s.handleLearnAnswer)
	})
}

// cacheStatic marks vendored assets as immutable — they only change when the
// binary does, and the binary is what serves them.
func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

// staticVersion hashes every embedded asset into one short token. It goes into
// asset URLs so the `immutable` cache header is safe: assets are only immutable
// for a given build, and a new build produces a new URL. Without it a deploy
// would leave browsers on the previous stylesheet for a year.
func staticVersion() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:10], nil
}

func templateFuncs(staticVersion string) template.FuncMap {
	return template.FuncMap{
		// asset appends the build token so caching can stay aggressive.
		"asset":  func(path string) string { return path + "?v=" + staticVersion },
		"levels": user.AllLevels,
		"levelReached": func(current, step user.Level) bool {
			return current.AtLeast(step)
		},
		// digits splits a number into fixed-width characters, one per flap of
		// the board. Numbers wider than width are not truncated — losing a digit
		// would misreport a score.
		"digits": func(n, width int) []string {
			s := strconv.Itoa(n)
			for len(s) < width {
				s = "0" + s
			}
			out := make([]string, 0, len(s))
			for _, r := range s {
				out = append(out, string(r))
			}
			return out
		},
		"inc": func(i int) int { return i + 1 },
		// filled renders a done-out-of-need tally as a slice of booleans, one
		// per flap.
		"filled": func(done, need int) []bool {
			out := make([]bool, need)
			for i := range out {
				out[i] = i < done
			}
			return out
		},
		// paragraphs splits stored theory on blank lines so a rule keeps its
		// structure instead of collapsing into one wall of text.
		"paragraphs": func(text string) []string {
			out := []string{}
			for _, p := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		},
		// plural picks the Russian form: 1 вопрос, 2 вопроса, 5 вопросов.
		// Without it the interface says "1 вопросов", which reads as broken.
		"plural": func(n int, one, few, many string) string {
			if mod100 := n % 100; mod100 >= 11 && mod100 <= 14 {
				return many
			}
			switch n % 10 {
			case 1:
				return one
			case 2, 3, 4:
				return few
			default:
				return many
			}
		},
		"percent": func(part, total int) int {
			if total <= 0 {
				return 0
			}
			return part * 100 / total
		},
		// chars splits a short string into one flap per character.
		"chars": func(v any) []string {
			out := []string{}
			for _, r := range fmt.Sprint(v) {
				out = append(out, string(r))
			}
			return out
		},
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("web: render failed", "template", name, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// renderError shows a failure in the interface's own voice: what happened and
// what to do, without leaking which internal step broke.
func (s *Server) renderError(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusInternalServerError)
	s.render(w, "error.html", map[string]string{"Message": message})
}
