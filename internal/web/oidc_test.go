package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestComputeChallengeMatchesRFC7636(t *testing.T) {
	verifier, err := GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}

	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])

	if got := ComputeChallenge(verifier); got != want {
		t.Fatalf("ComputeChallenge = %q, want %q", got, want)
	}
}

func TestGenerateVerifierIsUnpaddedURLSafe(t *testing.T) {
	v, err := GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(v); err != nil {
		t.Fatalf("verifier is not base64url without padding: %v", err)
	}
	// 32 random bytes → 43 chars, comfortably inside RFC 7636's 43..128 range.
	if len(v) != 43 {
		t.Fatalf("verifier length = %d, want 43", len(v))
	}
}

func TestGenerateVerifierIsRandom(t *testing.T) {
	a, _ := GenerateVerifier()
	b, _ := GenerateVerifier()
	if a == b {
		t.Fatal("two verifiers came out identical")
	}
}

func TestAuthorizationURLCarriesPKCEAndState(t *testing.T) {
	c := NewOIDCClient("https://oauth.example.org", "client-1", "secret", "https://app.example.org/auth/telegram/callback", "https://oauth.example.org")

	raw := c.AuthorizationURL("st4te", "ch4llenge")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Path != "/auth" {
		t.Fatalf("path = %q, want /auth", u.Path)
	}

	q := u.Query()
	for k, want := range map[string]string{
		"client_id":             "client-1",
		"response_type":         "code",
		"scope":                 "openid profile",
		"state":                 "st4te",
		"code_challenge":        "ch4llenge",
		"code_challenge_method": "S256",
		"redirect_uri":          "https://app.example.org/auth/telegram/callback",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	// The secret must never travel in the front-channel redirect.
	if q.Get("client_secret") != "" {
		t.Error("client_secret leaked into the authorization URL")
	}
}

// idToken builds an unsigned JWT with the given claims. The signature is
// irrelevant: Exchange deliberately does not verify it (the token arrives over
// TLS server-to-server), so the tests mirror that contract.
func idToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(payload) + ".sig"
}

// validClaims is a complete, well-formed claim set. Every strictness test below
// starts from it and breaks exactly one thing, so a failure names its own cause.
func validClaims() map[string]any {
	return map[string]any{
		"id":                 float64(777),
		"name":               "Алиса",
		"preferred_username": "alice",
		"picture":            "https://example.org/a.jpg",
		"iss":                "https://oauth.telegram.org",
		"aud":                "client-1",
		"iat":                float64(time.Now().Unix()),
		"exp":                float64(time.Now().Add(time.Hour).Unix()),
	}
}

func claimsWithout(key string) map[string]any {
	c := validClaims()
	delete(c, key)
	return c
}

func claimsWith(key string, value any) map[string]any {
	c := validClaims()
	c[key] = value
	return c
}

func newTestOIDCClient(baseURL string) *OIDCClient {
	return NewOIDCClient(baseURL, "client-1", "secret", "https://app/cb", "https://oauth.telegram.org")
}

func stubTokenEndpoint(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		// PKCE only works if the verifier actually reaches the token endpoint.
		if r.PostForm.Get("code_verifier") == "" {
			t.Error("code_verifier missing from token request")
		}
		if r.PostForm.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", r.PostForm.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestExchangeReadsProfileFromIDToken(t *testing.T) {
	tok := idToken(t, validClaims())
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{
		"access_token": "at", "token_type": "bearer", "id_token": tok,
	})

	c := newTestOIDCClient(srv.URL)
	u, err := c.Exchange(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if u.ID != 777 || u.FirstName != "Алиса" || u.Username != "alice" {
		t.Fatalf("got %+v", u)
	}
}

// Telegram is inconsistent about whether id is a number or a string.
func TestExchangeAcceptsStringID(t *testing.T) {
	tok := idToken(t, claimsWith("id", "777"))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	c := newTestOIDCClient(srv.URL)
	u, err := c.Exchange(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if u.ID != 777 {
		t.Fatalf("ID = %d, want 777", u.ID)
	}
}

func TestExchangeRejectsExpiredIDToken(t *testing.T) {
	tok := idToken(t, claimsWith("exp", float64(time.Now().Add(-time.Hour).Unix())))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	c := newTestOIDCClient(srv.URL)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected an error for an expired id_token")
	}
}

func TestExchangeRejectsAudienceMismatch(t *testing.T) {
	tok := idToken(t, claimsWith("aud", "someone-else"))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	c := newTestOIDCClient(srv.URL)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected an error when aud does not match our client id")
	}
}

func TestExchangeRejectsForeignIssuer(t *testing.T) {
	tok := idToken(t, claimsWith("iss", "https://evil.example.com"))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	c := newTestOIDCClient(srv.URL)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected an error for an issuer that is not Telegram")
	}
}

func TestExchangeRejectsMissingIDToken(t *testing.T) {
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"access_token": "at"})

	c := newTestOIDCClient(srv.URL)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected an error when the response carries no id_token")
	}
}

func TestExchangeRejectsMissingIDClaim(t *testing.T) {
	tok := idToken(t, claimsWithout("id"))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	c := newTestOIDCClient(srv.URL)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected an error when the id claim is absent")
	}
}

func TestExchangeReportsTokenEndpointFailure(t *testing.T) {
	srv := stubTokenEndpoint(t, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})

	c := newTestOIDCClient(srv.URL)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("expected an error for a non-200 token response")
	}
}

// --- phase 2: the claim checks become mandatory ------------------------------

func TestExchangeRequiresIssuerAudienceAndExpiry(t *testing.T) {
	// Until now each of these was enforced only when present, which left the
	// decision to whoever issued the token. Absence is now a refusal.
	for _, claim := range []string{"iss", "aud", "exp"} {
		tok := idToken(t, claimsWithout(claim))
		srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

		_, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier")
		if err == nil {
			t.Errorf("Exchange() accepted an id_token with no %s claim", claim)
			continue
		}
		if !strings.Contains(err.Error(), claim) {
			t.Errorf("error for a missing %s is %q and does not name the claim", claim, err)
		}
	}
}

func TestExchangeMatchesTheIssuerExactly(t *testing.T) {
	// The old check was `iss contains "telegram"`, which any host with that
	// substring satisfies — and which no honest non-Telegram issuer can pass.
	for name, iss := range map[string]string{
		"substring host":   "https://telegram-auth.example.com",
		"trailing slash":   "https://oauth.telegram.org/",
		"different scheme": "http://oauth.telegram.org",
		"suffix":           "https://oauth.telegram.org.evil.com",
	} {
		tok := idToken(t, claimsWith("iss", iss))
		srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

		if _, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier"); err == nil {
			t.Errorf("Exchange() accepted issuer %q (%s)", iss, name)
		}
	}
}

func TestExchangeAcceptsAConfiguredIssuerOtherThanTelegram(t *testing.T) {
	// The expected issuer is configuration, not a substring rule, so pointing
	// the client at another OIDC provider is a deliberate edit rather than
	// something that happens to pass.
	const otherIssuer = "https://id.example.com"
	tok := idToken(t, claimsWith("iss", otherIssuer))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	c := NewOIDCClient(srv.URL, "client-1", "secret", "https://app/cb", otherIssuer)
	if _, err := c.Exchange(context.Background(), "code", "verifier"); err != nil {
		t.Errorf("Exchange() = %v for the configured issuer", err)
	}
}

func TestExchangeHandlesAnAudienceArray(t *testing.T) {
	// `claims["aud"].(string)` returns "" for an array, which used to switch the
	// audience check off without a word. Arrays are now read properly.
	tok := idToken(t, claimsWith("aud", []any{"someone-else", "another"}))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})
	if _, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Error("Exchange() accepted an audience array that does not contain our client id")
	}

	tok = idToken(t, claimsWith("aud", []any{"someone-else", "client-1"}))
	srv = stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})
	if _, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier"); err != nil {
		t.Errorf("Exchange() = %v for an audience array that does contain our client id", err)
	}
}

func TestExchangeRejectsAnIssuedAtFromTheFuture(t *testing.T) {
	tok := idToken(t, claimsWith("iat", float64(time.Now().Add(5*time.Minute).Unix())))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	if _, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Error("Exchange() accepted an id_token issued five minutes from now")
	}
}

func TestExchangeToleratesSmallClockSkewOnIssuedAt(t *testing.T) {
	tok := idToken(t, claimsWith("iat", float64(time.Now().Add(30*time.Second).Unix())))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	if _, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier"); err != nil {
		t.Errorf("Exchange() = %v for 30s of clock skew, which is within tolerance", err)
	}
}

func TestExchangeKeepsLargeTelegramIDsExact(t *testing.T) {
	// encoding/json decodes into float64 by default, and ids above 2^53 come back
	// rounded — silently attaching the session to a neighbouring account.
	const huge = int64(9007199254740993) // 2^53 + 1
	tok := idToken(t, claimsWith("id", json.Number(strconv.FormatInt(huge, 10))))
	srv := stubTokenEndpoint(t, http.StatusOK, map[string]any{"id_token": tok})

	u, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if u.ID != huge {
		t.Errorf("ID = %d, want %d — precision was lost through float64", u.ID, huge)
	}
}

func TestExchangeDoesNotFollowRedirects(t *testing.T) {
	// Go's default client follows up to ten redirects. A token endpoint that
	// answers 302 would have the client read an id_token from wherever it landed,
	// which is a live bug whichever provider the client points at.
	var attacker *httptest.Server
	attacker = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id_token": idToken(t, claimsWith("id", float64(66666))),
		})
	}))
	t.Cleanup(attacker.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, nil, attacker.URL+"/token", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	u, err := newTestOIDCClient(srv.URL).Exchange(context.Background(), "code", "verifier")
	if err == nil {
		t.Fatalf("Exchange() followed a redirect and returned user %+v", u)
	}
}
