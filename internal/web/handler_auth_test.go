package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"makeMe/internal/domain/user"
)

// fakeUserRepo is an in-memory stand-in for the Postgres repository. Create is
// an upsert keyed by telegram_id, matching the real CreateUser query — that
// idempotency is what makes a bot user and a web user the same row.
type fakeUserRepo struct {
	byTelegram map[int64]*user.User
	byID       map[int64]*user.User
	nextID     int64
	failCreate bool
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byTelegram: map[int64]*user.User{},
		byID:       map[int64]*user.User{},
		nextID:     1,
	}
}

func (f *fakeUserRepo) Create(_ context.Context, telegramID int64, username, firstName string) (*user.User, error) {
	if f.failCreate {
		return nil, fmt.Errorf("boom")
	}
	if u, ok := f.byTelegram[telegramID]; ok {
		u.Username, u.FirstName = username, firstName
		return u, nil
	}
	u := &user.User{
		ID: f.nextID, TelegramID: telegramID,
		Username: username, FirstName: firstName,
		Level: user.A1,
	}
	f.nextID++
	f.byTelegram[telegramID] = u
	f.byID[u.ID] = u
	return u, nil
}

func (f *fakeUserRepo) GetByTelegramID(_ context.Context, telegramID int64) (*user.User, error) {
	if u, ok := f.byTelegram[telegramID]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (f *fakeUserRepo) GetByID(_ context.Context, id int64) (*user.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (f *fakeUserRepo) UpdateLevel(_ context.Context, id int64, level user.Level) error {
	if u, ok := f.byID[id]; ok {
		u.Level, u.Onboarded = level, true
		return nil
	}
	return user.ErrNotFound
}

func (f *fakeUserRepo) ListAll(context.Context) ([]user.User, error) { return nil, nil }

type testEnv struct {
	router *chi.Mux
	repo   *fakeUserRepo
	jwt    *JWTService
	oidc   *httptest.Server
}

// newTestEnv builds the web server against a stub OIDC provider that always
// returns the given Telegram identity.
func newTestEnv(t *testing.T, tgID int64, name, username string) *testEnv {
	t.Helper()

	oidcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
		claims, _ := json.Marshal(map[string]any{
			"id": float64(tgID), "name": name, "preferred_username": username,
			"iss": "https://oauth.telegram.org", "aud": "client-1",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token": enc(`{"alg":"none"}`) + "." + enc(string(claims)) + ".sig",
		})
	}))
	t.Cleanup(oidcSrv.Close)

	repo := newFakeUserRepo()
	jwtSvc, err := NewJWTService(testSecret(t), 24)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}

	srv, err := New(
		Deps{UserSvc: user.NewService(repo)},
		jwtSvc,
		NewOIDCClient(oidcSrv.URL, "client-1", "secret", "https://app.test/auth/telegram/callback"),
		false, // no TLS in tests, so Secure cookies would be dropped
	)
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	r := chi.NewRouter()
	srv.Mount(r)
	return &testEnv{router: r, repo: repo, jwt: jwtSvc, oidc: oidcSrv}
}

func (e *testEnv) do(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func cookieFrom(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range (&http.Response{Header: rr.Header()}).Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestLoginPageRenders(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Войти через Telegram") {
		t.Error("login page is missing the sign-in action")
	}
	// The design forbids emoji in the web UI; catch a regression cheaply.
	for _, e := range []string{"🎓", "🔥", "✅", "❌", "📚"} {
		if strings.Contains(body, e) {
			t.Errorf("login page contains emoji %q — the web UI must not use them", e)
		}
	}
}

func TestAuthStartSetsPKCECookiesAndRedirects(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/auth/telegram", nil))

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}

	state := cookieFrom(rr, stateCookie)
	verifier := cookieFrom(rr, verifierCookie)
	if state == nil || verifier == nil {
		t.Fatal("state and verifier cookies must both be set")
	}
	if !state.HttpOnly || !verifier.HttpOnly {
		t.Error("PKCE cookies must be HttpOnly")
	}
	if state.SameSite != http.SameSiteLaxMode {
		t.Error("cookies must be SameSite=Lax — that is the CSRF defence here")
	}

	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	q := loc.Query()
	if q.Get("state") != state.Value {
		t.Error("state in the redirect does not match the cookie")
	}
	if q.Get("code_challenge") != ComputeChallenge(verifier.Value) {
		t.Error("code_challenge is not the S256 hash of the verifier cookie")
	}
}

func TestCallbackHappyPathCreatesUserAndSession(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")

	req := httptest.NewRequest(http.MethodGet, "/auth/telegram/callback?code=abc&state=s1", nil)
	req.AddCookie(&http.Cookie{Name: stateCookie, Value: "s1"})
	req.AddCookie(&http.Cookie{Name: verifierCookie, Value: "v1"})
	rr := env.do(t, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want /", got)
	}

	auth := cookieFrom(rr, authCookie)
	if auth == nil || auth.Value == "" {
		t.Fatal("auth_token cookie was not set")
	}
	if id := env.jwt.UserID(auth.Value); id == 0 {
		t.Fatal("auth_token does not verify")
	}
	if u, err := env.repo.GetByTelegramID(context.Background(), 777); err != nil || u.FirstName != "Алиса" {
		t.Fatalf("user was not created from the Telegram identity: %v", err)
	}
}

// A user who started in the bot must land on the same row when they sign in on
// the web — this is the whole basis of the cross-channel sync.
func TestCallbackReusesExistingBotUser(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")

	existing, err := env.repo.Create(context.Background(), 777, "alice", "Алиса")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	existing.Level, existing.Onboarded = user.B2, true

	req := httptest.NewRequest(http.MethodGet, "/auth/telegram/callback?code=abc&state=s1", nil)
	req.AddCookie(&http.Cookie{Name: stateCookie, Value: "s1"})
	req.AddCookie(&http.Cookie{Name: verifierCookie, Value: "v1"})
	rr := env.do(t, req)

	auth := cookieFrom(rr, authCookie)
	if auth == nil {
		t.Fatal("auth_token cookie was not set")
	}
	if got := env.jwt.UserID(auth.Value); got != existing.ID {
		t.Fatalf("session points at user %d, want the existing %d", got, existing.ID)
	}
	if len(env.repo.byID) != 1 {
		t.Fatalf("expected exactly one user row, got %d", len(env.repo.byID))
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")

	req := httptest.NewRequest(http.MethodGet, "/auth/telegram/callback?code=abc&state=attacker", nil)
	req.AddCookie(&http.Cookie{Name: stateCookie, Value: "s1"})
	req.AddCookie(&http.Cookie{Name: verifierCookie, Value: "v1"})
	rr := env.do(t, req)

	if got := rr.Header().Get("Location"); got != "/login?error=state" {
		t.Fatalf("Location = %q, want /login?error=state", got)
	}
	if c := cookieFrom(rr, authCookie); c != nil && c.Value != "" {
		t.Fatal("a session was issued despite the state mismatch")
	}
}

func TestCallbackRejectsMissingVerifierCookie(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")

	req := httptest.NewRequest(http.MethodGet, "/auth/telegram/callback?code=abc&state=s1", nil)
	req.AddCookie(&http.Cookie{Name: stateCookie, Value: "s1"})
	rr := env.do(t, req)

	if got := rr.Header().Get("Location"); got != "/login?error=state" {
		t.Fatalf("Location = %q, want /login?error=state", got)
	}
}

func TestCallbackHandlesUserCancellation(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/auth/telegram/callback?error=access_denied", nil))

	if got := rr.Header().Get("Location"); got != "/login?error=cancelled" {
		t.Fatalf("Location = %q, want /login?error=cancelled", got)
	}
}

func TestProtectedPageRedirectsAnonymousToLogin(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want /login", got)
	}
}

// htmx must be told to navigate; swapping a login page into a fragment would
// leave the user on a half-broken screen.
func TestProtectedFragmentReturns401WithRedirectHint(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	rr := env.do(t, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if got := rr.Header().Get("HX-Redirect"); got != "/login" {
		t.Fatalf("HX-Redirect = %q, want /login", got)
	}
}

func TestProtectedPageServesAuthenticatedUser(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")

	u, err := env.repo.Create(context.Background(), 777, "alice", "Алиса")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	token, err := env.jwt.Create(u.ID)
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
	rr := env.do(t, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Алиса") {
		t.Error("home page does not greet the signed-in user")
	}
}

// A token that survives its user (deleted row) must not wedge the browser in a
// redirect loop — the stale cookie gets cleared on the way out.
func TestStaleTokenForUnknownUserClearsCookie(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")

	token, err := env.jwt.Create(4242)
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
	rr := env.do(t, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if c := cookieFrom(rr, authCookie); c == nil || c.MaxAge >= 0 {
		t.Fatal("the stale auth cookie was not cleared")
	}
}

func TestLoginRedirectsAlreadySignedInUser(t *testing.T) {
	env := newTestEnv(t, 777, "Алиса", "alice")

	u, _ := env.repo.Create(context.Background(), 777, "alice", "Алиса")
	token, _ := env.jwt.Create(u.ID)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.AddCookie(&http.Cookie{Name: authCookie, Value: token})
	rr := env.do(t, req)

	if got := rr.Header().Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want /", got)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/logout", nil))

	c := cookieFrom(rr, authCookie)
	if c == nil || c.MaxAge >= 0 {
		t.Fatal("logout did not clear auth_token")
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want /login", got)
	}
}

func TestSecurityHeadersAndCSP(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/login", nil))

	csp := rr.Header().Get("Content-Security-Policy")
	// The strict CSP is only affordable because every asset is self-hosted; if
	// a CDN ever creeps back in, this assertion is what catches it.
	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"base-uri 'none'",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP is missing %q: %q", directive, csp)
		}
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options is not set")
	}
}

// 'unsafe-inline' in script-src would undo most of the XSS protection the rest
// of this setup buys. Assert on script-src specifically — style-src is allowed
// to keep it, and a naive substring check over the whole header would not tell
// the two apart.
func TestScriptSrcHasNoUnsafeInline(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	csp := env.do(t, httptest.NewRequest(http.MethodGet, "/login", nil)).
		Header().Get("Content-Security-Policy")

	for _, directive := range strings.Split(csp, ";") {
		d := strings.TrimSpace(directive)
		if !strings.HasPrefix(d, "script-src") {
			continue
		}
		if strings.Contains(d, "unsafe-inline") || strings.Contains(d, "unsafe-eval") {
			t.Fatalf("script-src must stay strict, got %q", d)
		}
		return
	}
	t.Fatal("CSP declares no script-src directive")
}

// With script-src 'self' the pages must carry no executable markup at all —
// an inline <script> or an on*= handler would silently stop working.
func TestPagesCarryNoInlineScriptsOrHandlers(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")

	u, _ := env.repo.Create(context.Background(), 777, "alice", "Алиса")
	token, _ := env.jwt.Create(u.ID)

	pages := map[string]*http.Request{
		"/login": httptest.NewRequest(http.MethodGet, "/login", nil),
		"/":      httptest.NewRequest(http.MethodGet, "/", nil),
	}
	pages["/"].AddCookie(&http.Cookie{Name: authCookie, Value: token})

	for name, req := range pages {
		body := env.do(t, req).Body.String()

		// An inline block is a <script> without src=.
		for _, chunk := range strings.Split(body, "<script")[1:] {
			tag := chunk
			if i := strings.Index(chunk, ">"); i >= 0 {
				tag = chunk[:i]
			}
			if !strings.Contains(tag, "src=") {
				t.Errorf("%s carries an inline <script> — move it to static/js/", name)
			}
		}
		for _, attr := range []string{"onclick=", "onload=", "onchange=", "onsubmit=", "hx-on"} {
			if strings.Contains(body, attr) {
				t.Errorf("%s carries %q — script-src 'self' blocks it", name, attr)
			}
		}
	}
}

func TestVendoredAssetsAreServedLocally(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")

	for _, path := range []string{
		"/static/app.css",
		"/static/tailwind.css",
		"/static/vendor/htmx.min.js",
		"/static/vendor/fonts.css",
	} {
		rr := env.do(t, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Errorf("%s → %d, want 200", path, rr.Code)
		}
		if rr.Body.Len() == 0 {
			t.Errorf("%s served an empty body", path)
		}
	}
}

// The Tailwind Play CDN build ships a compiler and generates CSS in the browser
// on every load; Tailwind documents it as development-only. We ship a
// pre-built stylesheet instead, so the compiler must never reappear — whether
// from a CDN or vendored into static/.
func TestTailwindIsPrebuiltNotTheBrowserCompiler(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")

	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/static/vendor/tailwind.js", nil))
	if rr.Code == http.StatusOK {
		t.Error("the Tailwind browser compiler is being served — ship the built CSS instead")
	}

	css := env.do(t, httptest.NewRequest(http.MethodGet, "/static/tailwind.css", nil))
	body := css.Body.String()
	if !strings.Contains(body, "box-sizing") {
		t.Error("built stylesheet is missing Preflight — the layout relies on it")
	}
	// A browser-compiler bundle would be hundreds of KB; the built sheet for
	// this template set is a few KB. Catch an accidental swap by size.
	if css.Body.Len() > 200_000 {
		t.Errorf("tailwind.css is %d bytes — that looks like the compiler bundle, not built output", css.Body.Len())
	}
}

// The design system forbids rounded corners and shadows, and tailwind.config
// zeroes those scales. If someone re-enables them, the utilities reappear in
// the built CSS — fail before that reaches a template.
func TestBuiltCSSHasNoRadiusOrShadowUtilities(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	body := env.do(t, httptest.NewRequest(http.MethodGet, "/static/tailwind.css", nil)).Body.String()

	for _, cls := range []string{".rounded-lg", ".rounded-md", ".shadow-md", ".shadow-lg"} {
		if strings.Contains(body, cls) {
			t.Errorf("built CSS contains %s — the design system forbids radii and shadows", cls)
		}
	}
}

// The pages must not reach out to any third-party origin — that is the whole
// point of vendoring, and it is what lets the CSP stay strict.
func TestPagesReferenceNoExternalOrigins(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/login", nil))

	for _, host := range []string{"cdn.tailwindcss.com", "unpkg.com", "fonts.googleapis.com", "fonts.gstatic.com"} {
		if strings.Contains(rr.Body.String(), host) {
			t.Errorf("page references external host %q", host)
		}
	}
}

// A mistyped URL must land on the site's own 404, not chi's bare text page —
// that one has no navigation and no way back.
func TestUnknownURLRendersStyled404(t *testing.T) {
	env := newTestEnv(t, 1, "Алиса", "alice")
	rr := env.do(t, httptest.NewRequest(http.MethodGet, "/no-such-page", nil))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "404 page not found") {
		t.Error("chi's bare 404 is being served")
	}
	if !strings.Contains(body, "Такой страницы нет") {
		t.Error("the styled 404 page was not rendered")
	}
	if !strings.Contains(body, "/tasks") {
		t.Error("the 404 page offers no way back")
	}
}

// Russian plurals: "1 вопросов" reads as broken software, and the counts on
// the grammar index are user-facing.
func TestPluralHelperPicksRussianForms(t *testing.T) {
	fn := templateFuncs("v")["plural"].(func(int, string, string, string) string)

	cases := map[int]string{
		1: "вопрос", 2: "вопроса", 4: "вопроса", 5: "вопросов",
		11: "вопросов", 12: "вопросов", 14: "вопросов",
		21: "вопрос", 22: "вопроса", 25: "вопросов",
		101: "вопрос", 111: "вопросов",
		0: "вопросов",
	}
	for n, want := range cases {
		if got := fn(n, "вопрос", "вопроса", "вопросов"); got != want {
			t.Errorf("plural(%d) = %q, want %q", n, got, want)
		}
	}
}
