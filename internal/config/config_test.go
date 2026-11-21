package config

import (
	"strings"
	"testing"
	"time"
)

// setBaseEnv sets what Load needs regardless of the webhook, and clears every
// variable the tests below vary, so a developer's shell cannot leak into them.
func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "123:abc")
	t.Setenv("WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("TELEGRAM_LOCAL_MODE", "")
	t.Setenv("TELEGRAM_INIT_TIMEOUT", "")
}

func TestLoad_EmptyWebhookURLWithoutLocalModeFails(t *testing.T) {
	setBaseEnv(t)
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "WEBHOOK_URL is required") ||
		!strings.Contains(err.Error(), "TELEGRAM_LOCAL_MODE=true") {
		t.Fatalf("got %v, want an error naming WEBHOOK_URL and the way out", err)
	}
}

func TestLoad_LocalModeWithoutWebhookURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TELEGRAM_LOCAL_MODE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.TelegramLocalMode || cfg.WebhookURL != "" {
		t.Fatalf("local=%v url=%q", cfg.TelegramLocalMode, cfg.WebhookURL)
	}
}

func TestLoad_LocalModeFalseIsNotLocal(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TELEGRAM_LOCAL_MODE", "false")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "WEBHOOK_URL is required") {
		t.Fatalf("got %v, want WEBHOOK_URL is required", err)
	}
}

func TestLoad_LocalModeWithWebhookURLFails(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TELEGRAM_LOCAL_MODE", "true")
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "abc")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("got %v, want a mutually-exclusive error", err)
	}
}

// A typo must not read as "false" and then fail with an unrelated complaint
// about WEBHOOK_URL: the error names the flag that was mistyped.
func TestLoad_LocalModeTypoFails(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TELEGRAM_LOCAL_MODE", "ture")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "TELEGRAM_LOCAL_MODE must be true or false") {
		t.Fatalf("got %v, want a TELEGRAM_LOCAL_MODE parse error", err)
	}
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

func TestLoad_TelegramInitTimeout(t *testing.T) {
	cases := []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{"", DefaultTelegramInitTimeout, true},
		{"45s", 45 * time.Second, true},
		{"1m", time.Minute, true},
		{"30", 0, false}, // no unit: an error, not 30ns and not the default
		{"0s", 0, false},
		{"-5s", 0, false},
		{"soon", 0, false},
	}
	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("TELEGRAM_LOCAL_MODE", "true")
			t.Setenv("TELEGRAM_INIT_TIMEOUT", c.value)
			cfg, err := Load()
			if !c.ok {
				if err == nil || !strings.Contains(err.Error(), "TELEGRAM_INIT_TIMEOUT") {
					t.Fatalf("got %v, want a TELEGRAM_INIT_TIMEOUT error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.TelegramInitTimeout != c.want {
				t.Fatalf("timeout = %v, want %v", cfg.TelegramInitTimeout, c.want)
			}
		})
	}
}
