package web

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func testSecret(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestNewJWTService_RejectsShortSecret(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := NewJWTService(short, 24); err == nil {
		t.Fatal("expected an error for a secret shorter than 32 bytes")
	}
}

func TestNewJWTService_RejectsNonBase64(t *testing.T) {
	if _, err := NewJWTService("not!base64!!", 24); err == nil {
		t.Fatal("expected an error for a non-base64 secret")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	s, err := NewJWTService(testSecret(t), 24)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}

	token, err := s.Create(42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := s.UserID(token); got != 42 {
		t.Fatalf("UserID = %d, want 42", got)
	}
}

func TestJWTRejectsForeignSignature(t *testing.T) {
	mine, _ := NewJWTService(testSecret(t), 24)
	theirs, _ := NewJWTService(base64.StdEncoding.EncodeToString([]byte("ffffffffffffffffffffffffffffffff")), 24)

	token, err := theirs.Create(42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := mine.UserID(token); got != 0 {
		t.Fatalf("UserID = %d, want 0 for a token signed with another key", got)
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	// Negative TTL is coerced to the 24h default, so build the expired token by
	// hand-rolling a service with a tiny TTL instead.
	s, err := NewJWTService(testSecret(t), 24)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	s.ttl = -time.Hour

	token, err := s.Create(42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := s.UserID(token); got != 0 {
		t.Fatalf("UserID = %d, want 0 for an expired token", got)
	}
}

func TestJWTRejectsGarbage(t *testing.T) {
	s, _ := NewJWTService(testSecret(t), 24)
	for _, tok := range []string{"", "abc", "a.b.c", strings.Repeat("x", 200)} {
		if got := s.UserID(tok); got != 0 {
			t.Fatalf("UserID(%q) = %d, want 0", tok, got)
		}
	}
}

// The "alg":"none" downgrade is the classic JWT attack: a token whose header
// claims no signature must never be accepted.
func TestJWTRejectsAlgNone(t *testing.T) {
	s, _ := NewJWTService(testSecret(t), 24)

	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	forged := enc(`{"alg":"none","typ":"JWT"}`) + "." + enc(`{"sub":"42"}`) + "."

	if got := s.UserID(forged); got != 0 {
		t.Fatalf("UserID = %d, want 0 for an alg=none token", got)
	}
}

func TestJWTTTLMatchesConfig(t *testing.T) {
	s, err := NewJWTService(testSecret(t), 3)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	if s.TTL() != 3*time.Hour {
		t.Fatalf("TTL = %v, want 3h", s.TTL())
	}
}
