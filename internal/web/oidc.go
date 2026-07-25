package web

import (
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
	http         *http.Client
}

func NewOIDCClient(baseURL, clientID, clientSecret, redirectURI string) *OIDCClient {
	return &OIDCClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		http:         &http.Client{Timeout: 15 * time.Second},
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
// The signature is intentionally NOT verified: the token came straight from
// Telegram's token endpoint over TLS in a server-to-server call, so the
// transport already guarantees integrity and there is no public JWKS to check
// against. The claim checks below are defence in depth, and each is enforced
// only when the claim is present — this OIDC deployment is non-standard and
// omits some of them.
func (c *OIDCClient) parseIDToken(raw string) (*TelegramUser, error) {
	parts := strings.Split(raw, ".")
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected id_token format: got %d segments, want 3", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode id_token payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse id_token claims: %w", err)
	}

	if exp, ok := claims["exp"].(float64); ok && int64(exp) <= time.Now().Unix() {
		return nil, fmt.Errorf("id_token has expired")
	}
	if aud, ok := claims["aud"].(string); ok && aud != c.clientID {
		return nil, fmt.Errorf("id_token aud mismatch")
	}
	if iss, ok := claims["iss"].(string); ok && !strings.Contains(strings.ToLower(iss), "telegram") {
		return nil, fmt.Errorf("id_token iss mismatch: %s", iss)
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

// claimInt64 accepts both the JSON-number and string spellings of the id claim,
// since Telegram is not consistent about which it sends.
func claimInt64(v any) (int64, error) {
	switch t := v.(type) {
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
