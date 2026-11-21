package config

import (
	"strings"
	"testing"
)

// setBaseEnv sets what Load needs regardless of the webhook, and clears every
// variable the tests below vary, so a developer's shell cannot leak into them.
func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "123:abc")
	t.Setenv("WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
}

func TestLoad_WebhookURLWithoutSecretFails(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TELEGRAM_WEBHOOK_SECRET is required") {
		t.Fatalf("got %v, want an error naming TELEGRAM_WEBHOOK_SECRET", err)
	}
}

func TestLoad_WebhookURLWithSecret(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "abc_DEF-123")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TelegramWebhookSecret != "abc_DEF-123" {
		t.Fatalf("secret = %q", cfg.TelegramWebhookSecret)
	}
}

func TestLoad_WebhookSecretFormat(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		ok     bool
	}{
		{"one char", "a", true},
		{"256 chars", strings.Repeat("a", 256), true},
		{"hex from openssl", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
		{"257 chars", strings.Repeat("a", 257), false},
		{"base64 padding", "abc=", false},
		{"base64 plus", "a+b", false},
		{"slash", "a/b", false},
		{"space", "a b", false},
		{"non-ascii", "секрет", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("WEBHOOK_URL", "https://example.com/webhook")
			t.Setenv("TELEGRAM_WEBHOOK_SECRET", c.secret)
			_, err := Load()
			if c.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !c.ok && (err == nil || !strings.Contains(err.Error(), "1-256 characters")) {
				t.Fatalf("want a format error, got %v", err)
			}
		})
	}
}
