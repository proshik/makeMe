package web

import (
	"errors"
	"testing"

	"makeMe/internal/domain/user"
)

// Production once shipped with the hub created but never passed into Deps:
// /ws answered 404 and every open tab reconnected forever, with nothing in
// the logs. New must refuse that wiring so it fails at startup instead.
func TestNew_RequiresHub(t *testing.T) {
	jwtSvc, err := NewJWTService(testSecret(t), 24)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	oidc := NewOIDCClient("http://unused", "client-1", "secret", "http://app/cb", "https://oauth.telegram.org")

	srv, err := New(Deps{UserSvc: user.NewService(newFakeUserRepo())}, jwtSvc, oidc, false)
	if !errors.Is(err, ErrNoHub) {
		t.Fatalf("New without Hub: err = %v, want ErrNoHub", err)
	}
	if srv != nil {
		t.Fatal("New without Hub must not return a server")
	}
}
