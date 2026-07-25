package web

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"makeMe/internal/domain/user"
)

type ctxKey string

const userCtxKey ctxKey = "web_user"

const (
	authCookie     = "auth_token"
	stateCookie    = "oidc_state"
	verifierCookie = "oidc_code_verifier"
	// oidcCookieTTL bounds how long a half-finished login stays resumable.
	oidcCookieTTL = 5 * time.Minute
)

// UserFromContext returns the authenticated user, or nil when the request did
// not pass through requireAuth.
func UserFromContext(ctx context.Context) *user.User {
	u, _ := ctx.Value(userCtxKey).(*user.User)
	return u
}

// setCookie writes one of our cookies. SameSite=Lax is the CSRF defence for
// cookie auth: the browser will not attach these to cross-site state-changing
// requests, which is what lets the POST handlers skip a separate CSRF token.
func (s *Server) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// requireAuth resolves the auth_token cookie into a user. Page requests are
// redirected to /login; htmx and API requests get a 401 with a redirect hint so
// the client can navigate instead of swapping a login page into a fragment.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := s.jwt.UserID(cookieValue(r, authCookie))
		if userID == 0 {
			s.rejectUnauthenticated(w, r)
			return
		}

		u, err := s.userSvc.GetByID(r.Context(), userID)
		if err != nil {
			// Token is well-formed but the user is gone — treat as logged out
			// and drop the stale cookie so the loop doesn't repeat.
			slog.Warn("web: token references unknown user", "user_id", userID, "error", err)
			s.clearCookie(w, authCookie)
			s.rejectUnauthenticated(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) rejectUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if isFragmentRequest(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func isFragmentRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" || strings.HasPrefix(r.URL.Path, "/api/")
}

// contentSecurityPolicy is only affordable because every asset is served from
// our own origin — no CDN, no Google Fonts, Tailwind compiled ahead of time.
//
// `script-src 'self'` carries no 'unsafe-inline', which constrains the
// templates: no inline <script>, no on*="" handlers, no htmx hx-on / js:
// expressions. Put behaviour in static/js/app.js instead — the notes at the top
// of that file say the same thing to whoever edits it next.
//
// Two directives stay deliberately loose:
//   - style-src allows 'unsafe-inline': the templates carry inline style=""
//     attributes for design-token colours. Style injection is far less
//     dangerous than script execution, so this is an accepted trade-off rather
//     than a nonce pipeline.
//   - media-src allows https: because pronunciation mp3s are served straight
//     from the Free Dictionary API. Narrowing it to 'self' would mean proxying
//     every audio file through the app.
var contentSecurityPolicy = strings.Join([]string{
	"default-src 'self'",
	"script-src 'self'",
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data:",
	"font-src 'self'",
	"media-src 'self' https:",
	"connect-src 'self'",
	"form-action 'self'",
	"base-uri 'none'",
	"object-src 'none'",
	"frame-ancestors 'none'",
}, "; ")

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		next.ServeHTTP(w, r)
	})
}
