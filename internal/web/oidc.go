package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TelegramUser is what we need out of the id_token to identify the person.
// It deliberately carries only the Telegram identity — everything else about
// the user lives in our own users table.
type TelegramUser struct {
	ID        int64
	FirstName string
	Username  string
	PhotoURL  string
}

// OIDCClient implements Telegram's OAuth 2.0 authorization-code flow with PKCE.
// Telegram exposes no UserInfo endpoint, so the profile is read out of the
// id_token returned by the token endpoint.
type OIDCClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	redirectURI  string
	// expectedIssuer is the exact iss the id_token must carry. It is
	// configuration rather than a substring rule, so pointing this client at
	// something other than Telegram — a local login broker, say — is a
	// deliberate edit that fails loudly when it is wrong.
	expectedIssuer string
	http           *http.Client
}

func NewOIDCClient(baseURL, clientID, clientSecret, redirectURI, expectedIssuer string) *OIDCClient {
	return &OIDCClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		clientID:       clientID,
		clientSecret:   clientSecret,
		redirectURI:    redirectURI,
		expectedIssuer: expectedIssuer,
		http: &http.Client{
			Timeout: 15 * time.Second,
			// The default client follows up to ten redirects, which would have
			// it read an id_token from wherever the last hop pointed. There is
			// no legitimate redirect on a token endpoint.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return fmt.Errorf("token endpoint redirected to %s; refusing to follow", req.URL.Redacted())
			},
		},
	}
}

// GenerateVerifier returns a PKCE code_verifier: 32 random bytes, base64url
// without padding.
func GenerateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ComputeChallenge derives the S256 code_challenge from a verifier.
func ComputeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GenerateState returns an opaque anti-CSRF value for the authorization request.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (c *OIDCClient) AuthorizationURL(state, challenge string) string {
	q := url.Values{
		"client_id":             {c.clientID},
		"redirect_uri":          {c.redirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid profile"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return c.baseURL + "/auth?" + q.Encode()
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
}

// Exchange swaps the authorization code for tokens and extracts the user from
// the id_token.
func (c *OIDCClient) Exchange(ctx context.Context, code, verifier string) (*TelegramUser, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.redirectURI},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tr.IDToken == "" {
		return nil, fmt.Errorf("token response is missing id_token")
	}
	return c.parseIDToken(tr.IDToken)
}

// parseIDToken reads the claims out of the JWT payload.
//
// The signature is intentionally NOT verified: the token comes straight from the
// token endpoint over TLS in a server-to-server call, so the transport already
// guarantees integrity. Telegram does publish a JWKS at
// /.well-known/jwks.json, so verifying is possible and would be the stronger
// answer; not doing it is a decision, not an absence of means.
//
// That makes the claim checks below load-bearing rather than defence in depth,
// and every one of them is mandatory: enforcing a claim "only if present" hands
// the decision to whoever issued the token.
func (c *OIDCClient) parseIDToken(raw string) (*TelegramUser, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("unexpected id_token format: got %d segments, want 3", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode id_token payload: %w", err)
	}

	// UseNumber keeps integers exact. Through the default float64 a Telegram id
	// above 2^53 comes back rounded, which quietly attaches the session to a
	// neighbouring account.
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var claims map[string]any
	if err := dec.Decode(&claims); err != nil {
		return nil, fmt.Errorf("parse id_token claims: %w", err)
	}

	iss, _ := claims["iss"].(string)
	if iss == "" {
		return nil, fmt.Errorf("id_token has no iss claim")
	}
	// Exact equality, as OIDC Core 3.1.3.7 requires: a trailing slash makes a
	// different issuer identifier, and normalising it away is how a check turns
	// into an approximation.
	if iss != c.expectedIssuer {
		return nil, fmt.Errorf("id_token iss is %q, want %q", iss, c.expectedIssuer)
	}

	if err := checkAudience(claims["aud"], c.clientID); err != nil {
		return nil, err
	}

	exp, err := claimUnix(claims["exp"])
	if err != nil {
		return nil, fmt.Errorf("id_token exp claim: %w", err)
	}
	now := time.Now()
	if !now.Before(exp) {
		return nil, fmt.Errorf("id_token has expired")
	}
	// iat is optional in the sense that not every provider sends it, but when it
	// is there a token minted in the future is a sign something is wrong.
	if raw, present := claims["iat"]; present {
		iat, err := claimUnix(raw)
		if err != nil {
			return nil, fmt.Errorf("id_token iat claim: %w", err)
		}
		if iat.After(now.Add(clockSkew)) {
			return nil, fmt.Errorf("id_token iat is %s, which is in the future", iat)
		}
	}

	id, err := claimInt64(claims["id"])
	if err != nil {
		return nil, fmt.Errorf("id_token 'id' claim: %w", err)
	}

	return &TelegramUser{
		ID:        id,
		FirstName: claimString(claims["name"]),
		Username:  claimString(claims["preferred_username"]),
		PhotoURL:  claimString(claims["picture"]),
	}, nil
}

// clockSkew is how far ahead of us the issuer's clock may be.
const clockSkew = 60 * time.Second

// checkAudience accepts aud both as a string and as an array, which OIDC allows.
// Reading it only as a string leaves an array matching nothing, and a check that
// matches nothing is a check that is off.
func checkAudience(v any, clientID string) error {
	switch aud := v.(type) {
	case string:
		if aud != clientID {
			return fmt.Errorf("id_token aud is %q, want %q", aud, clientID)
		}
		return nil
	case []any:
		for _, entry := range aud {
			if s, ok := entry.(string); ok && s == clientID {
				return nil
			}
		}
		return fmt.Errorf("id_token aud %v does not contain %q", aud, clientID)
	case nil:
		return fmt.Errorf("id_token has no aud claim")
	default:
		return fmt.Errorf("id_token aud is a %T, want a string or an array of strings", v)
	}
}

// claimUnix reads a numeric date claim.
func claimUnix(v any) (time.Time, error) {
	switch t := v.(type) {
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return time.Time{}, fmt.Errorf("not a whole number: %q", t)
		}
		return time.Unix(n, 0), nil
	case float64:
		return time.Unix(int64(t), 0), nil
	case nil:
		return time.Time{}, fmt.Errorf("missing")
	default:
		return time.Time{}, fmt.Errorf("is a %T, want a number", v)
	}
}

// claimInt64 accepts both the JSON-number and string spellings of the id claim,
// since Telegram is not consistent about which it sends.
func claimInt64(v any) (int64, error) {
	switch t := v.(type) {
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, fmt.Errorf("not a whole number: %q", t)
		}
		return n, nil
	case float64:
		return int64(t), nil
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("not a number: %q", t)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("missing or of unexpected type %T", v)
	}
}

func claimString(v any) string {
	s, _ := v.(string)
	return s
}
