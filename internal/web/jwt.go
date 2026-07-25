package web

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTService issues and verifies the session tokens carried in the auth_token
// cookie. The token is stateless: logout clears the cookie but cannot revoke an
// already-issued token, which is why the TTL is kept short.
type JWTService struct {
	key []byte
	ttl time.Duration
}

// NewJWTService validates the secret eagerly so a misconfigured deployment fails
// at startup rather than on the first login attempt.
func NewJWTService(secretB64 string, ttlHours int) (*JWTService, error) {
	key, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil {
		return nil, fmt.Errorf("JWT_SECRET must be valid base64: %w", err)
	}
	if len(key) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must decode to at least 32 bytes for HS256, got %d", len(key))
	}
	if ttlHours <= 0 {
		ttlHours = 24
	}
	return &JWTService{key: key, ttl: time.Duration(ttlHours) * time.Hour}, nil
}

// TTL is exposed so the cookie max-age stays in sync with the token lifetime
// instead of the two drifting apart.
func (s *JWTService) TTL() time.Duration { return s.ttl }

func (s *JWTService) Create(userID int64) (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	})
	return tok.SignedString(s.key)
}

// UserID returns 0 for any token that is malformed, expired, or signed with the
// wrong key — callers treat 0 as "not authenticated" and never see the reason,
// which keeps the failure paths uniform.
func (s *JWTService) UserID(token string) int64 {
	parsed, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return s.key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return 0
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return 0
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
