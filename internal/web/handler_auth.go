package web

import (
	"log/slog"
	"net/http"
)

// loginErrors maps the ?error= codes we redirect with onto user-facing text.
// The interface explains what happened and what to do; it does not apologise
// and does not leak which internal step failed.
var loginErrors = map[string]string{
	"cancelled": "Вход отменён. Нажмите «Войти через Telegram», чтобы попробовать снова.",
	"state":     "Не удалось подтвердить вход — возможно, страница была открыта слишком долго. Попробуйте ещё раз.",
	"server":    "Telegram не ответил. Попробуйте войти через минуту.",
}

type loginPage struct {
	Error string
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Already signed in — no reason to show the door again.
	if s.jwt.UserID(cookieValue(r, authCookie)) != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login.html", loginPage{Error: loginErrors[r.URL.Query().Get("error")]})
}

// handleAuthStart begins the authorization-code flow: it mints state and a PKCE
// verifier, parks both in short-lived HttpOnly cookies, and sends the browser
// to Telegram.
func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	state, err := GenerateState()
	if err != nil {
		slog.Error("web: generate state", "error", err)
		http.Redirect(w, r, "/login?error=server", http.StatusSeeOther)
		return
	}
	verifier, err := GenerateVerifier()
	if err != nil {
		slog.Error("web: generate verifier", "error", err)
		http.Redirect(w, r, "/login?error=server", http.StatusSeeOther)
		return
	}

	s.setCookie(w, stateCookie, state, oidcCookieTTL)
	s.setCookie(w, verifierCookie, verifier, oidcCookieTTL)

	http.Redirect(w, r, s.oidc.AuthorizationURL(state, ComputeChallenge(verifier)), http.StatusSeeOther)
}

// handleAuthCallback completes the flow: verify state, exchange the code, then
// resolve the Telegram identity to one of our users.
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if q.Get("error") != "" {
		http.Redirect(w, r, "/login?error=cancelled", http.StatusSeeOther)
		return
	}

	code := q.Get("code")
	state := q.Get("state")
	savedState := cookieValue(r, stateCookie)
	verifier := cookieValue(r, verifierCookie)

	// The cookies are single-use regardless of the outcome.
	s.clearCookie(w, stateCookie)
	s.clearCookie(w, verifierCookie)

	if code == "" || state == "" || savedState == "" || verifier == "" || state != savedState {
		slog.Warn("web: rejected oauth callback", "has_code", code != "", "state_match", state == savedState)
		http.Redirect(w, r, "/login?error=state", http.StatusSeeOther)
		return
	}

	tgUser, err := s.oidc.Exchange(r.Context(), code, verifier)
	if err != nil {
		slog.Error("web: token exchange failed", "error", err)
		http.Redirect(w, r, "/login?error=server", http.StatusSeeOther)
		return
	}

	// Register is idempotent by telegram_id, so a user who started in the bot
	// lands on their existing row here — that is the whole basis of the
	// cross-channel sync.
	u, err := s.userSvc.Register(r.Context(), tgUser.ID, tgUser.Username, tgUser.FirstName)
	if err != nil {
		slog.Error("web: register user", "telegram_id", tgUser.ID, "error", err)
		http.Redirect(w, r, "/login?error=server", http.StatusSeeOther)
		return
	}

	token, err := s.jwt.Create(u.ID)
	if err != nil {
		slog.Error("web: create session token", "user_id", u.ID, "error", err)
		http.Redirect(w, r, "/login?error=server", http.StatusSeeOther)
		return
	}

	s.setCookie(w, authCookie, token, s.jwt.TTL())
	slog.Info("web: user signed in", "user_id", u.ID, "telegram_id", tgUser.ID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearCookie(w, authCookie)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
