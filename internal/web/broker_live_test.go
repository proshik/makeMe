package web

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This walks the real login against a real telegram-login-broker, using this
// package's own OIDCClient rather than a stub. Everything else in this file's
// neighbourhood tests our parser against tokens we made up ourselves; here the
// token comes from another program that was written to a specification, which is
// the only way to catch the two of us reading that specification differently.
//
// Skipped unless BROKER_URL is set, so `go test ./...` stays hermetic:
//
//	BROKER_URL=http://127.0.0.1:8091 \
//	BROKER_CLIENT_ID=makeme BROKER_CLIENT_SECRET=dev-secret \
//	go test ./internal/web -run TestLoginAgainstALiveBroker -v
//
// Start the broker in mock mode first; see its README.
func TestLoginAgainstALiveBroker(t *testing.T) {
	brokerURL := os.Getenv("BROKER_URL")
	if brokerURL == "" {
		t.Skip("BROKER_URL is not set; this test needs a running telegram-login-broker")
	}
	clientID := envOr("BROKER_CLIENT_ID", "makeme")
	clientSecret := envOr("BROKER_CLIENT_SECRET", "dev-secret")
	const redirectURI = "http://localhost:8081/auth/telegram/callback"

	// The broker's issuer is deliberately not Telegram's, and it is not derivable
	// from the base URL either — pointing an application at a broker has to be a
	// deliberate act, so it is configured, not guessed.
	client := NewOIDCClient(brokerURL, clientID, clientSecret, redirectURI, envOr("BROKER_ISSUER", brokerURL))

	verifier, err := GenerateVerifier()
	if err != nil {
		t.Fatal(err)
	}
	state, err := GenerateState()
	if err != nil {
		t.Fatal(err)
	}

	// A cookie jar, because the broker binds the browser that started a login to
	// the one that finishes it.
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	browser := &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// 1. Where the application would send the browser.
	resp, err := browser.Get(client.AuthorizationURL(state, ComputeChallenge(verifier)))
	if err != nil {
		t.Fatalf("GET /auth: %v", err)
	}
	page := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /auth = %d: %s", resp.StatusCode, page)
	}

	// 2. Pick an identity on the broker's mock page.
	flow := firstSubmatch(t, `name="flow" value="([^"]+)"`, page, "flow id")
	id := firstSubmatch(t, `<span class="id">(\d+)</span>`, page, "a mock user id")

	resp, err = browser.PostForm(strings.TrimRight(brokerURL, "/")+"/auth/select",
		url.Values{"flow": {flow}, "id": {id}})
	if err != nil {
		t.Fatalf("POST /auth/select: %v", err)
	}
	readAll(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("POST /auth/select = %d, want 302", resp.StatusCode)
	}

	// 3. What the browser hands back to the application's callback.
	callback, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := callback.Query().Get("state"); got != state {
		t.Errorf("state = %q, want the one we sent (%q)", got, state)
	}
	code := callback.Query().Get("code")
	if code == "" {
		t.Fatal("the broker sent no code back")
	}
	if base := strings.TrimSuffix(callback.String(), "?"+callback.RawQuery); base != redirectURI {
		t.Errorf("landed at %q, want %q", base, redirectURI)
	}

	// 4. The exchange our production code actually performs.
	user, err := client.Exchange(context.Background(), code, verifier)
	if err != nil {
		t.Fatalf("Exchange against the live broker: %v", err)
	}
	if user.ID == 0 {
		t.Error("no Telegram id came out of the id_token")
	}
	if got := strconv.FormatInt(user.ID, 10); got != id {
		t.Errorf("Telegram id = %s, want the identity that was chosen (%s)", got, id)
	}
	t.Logf("logged in as id=%d name=%q username=%q photo=%q",
		user.ID, user.FirstName, user.Username, user.PhotoURL)

	// 5. The code is spent. A broker that let it be replayed would be worse than
	// no broker at all.
	if _, err := client.Exchange(context.Background(), code, verifier); err == nil {
		t.Error("the same code was exchanged twice")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func firstSubmatch(t *testing.T, pattern, s, what string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("the broker's page carries no %s", what)
	}
	return m[1]
}
