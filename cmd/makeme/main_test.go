package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"

	"makeMe/internal/config"
	"makeMe/internal/telegram"
)

// recordingBotAPI stands in for api.telegram.org and records every method
// called on it. Handlers run synchronously within the call, so the record is
// complete as soon as configureWebhook returns.
type recordingBotAPI struct {
	mu      sync.Mutex
	methods []string
}

func (a *recordingBotAPI) called() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.methods...)
}

func newRecordingBot(t *testing.T) (*telegram.Bot, *recordingBotAPI) {
	t.Helper()
	api := &recordingBotAPI{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.methods = append(api.methods, r.URL.Path)
		api.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":true}`)
	}))
	t.Cleanup(srv.Close)
	b, err := telegram.New("test-token", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		bot.WithSkipGetMe(), bot.WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("telegram.New: %v", err)
	}
	return b, api
}

func TestConfigureWebhook_LocalModeLeavesWebhookAlone(t *testing.T) {
	b, api := newRecordingBot(t)
	cfg := &config.Config{TelegramLocalMode: true}
	if err := configureWebhook(context.Background(), cfg, b); err != nil {
		t.Fatalf("configureWebhook: %v", err)
	}
	if got := api.called(); len(got) != 0 {
		t.Fatalf("local mode called the Bot API: %v", got)
	}
}

func TestConfigureWebhook_SetsWebhookOutsideLocalMode(t *testing.T) {
	b, api := newRecordingBot(t)
	cfg := &config.Config{WebhookURL: "https://example.com/webhook", TelegramWebhookSecret: "abc"}
	if err := configureWebhook(context.Background(), cfg, b); err != nil {
		t.Fatalf("configureWebhook: %v", err)
	}
	if got := api.called(); len(got) != 1 || got[0] != "/bottest-token/setWebhook" {
		t.Fatalf("Bot API calls = %v, want one setWebhook", got)
	}
}

// bot.New calls getMe before it returns, so the only way the configured
// timeout takes effect is as an option handed to it. The fake Bot API answers
// getMe after 300ms: a 50ms timeout must fail, which it would not if the option
// were dropped and the library's 5s applied; a 5s timeout must succeed.
func TestBotOptions_CarryInitTimeoutIntoGetMe(t *testing.T) {
	const delay = 300 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"T","username":"t_bot"}}`)
	}))
	defer srv.Close()

	newBot := func(timeout time.Duration) error {
		cfg := &config.Config{TelegramInitTimeout: timeout}
		opts := append(botOptions(cfg), bot.WithServerURL(srv.URL))
		_, err := telegram.New("test-token", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, opts...)
		return err
	}

	start := time.Now()
	err := newBot(50 * time.Millisecond)
	if err == nil {
		t.Fatal("getMe slower than the configured timeout succeeded: the timeout did not reach bot.New")
	}
	if elapsed := time.Since(start); elapsed >= delay {
		t.Fatalf("failed after %v, want before the %v reply", elapsed, delay)
	}
	if err := newBot(5 * time.Second); err != nil {
		t.Fatalf("getMe within the configured timeout failed: %v", err)
	}
}

// Webhook settings are checked by the server, not by config.Load (the seed
// commands share Load), so run itself has to refuse them, and before
// migrations: DATABASE_URL points nowhere, and reaching it would fail with a
// connection error instead.
func TestRun_RejectsWebhookSettingsBeforeMigrations(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x:y@127.0.0.1:1/z?sslmode=disable")
	t.Setenv("TELEGRAM_TOKEN", "1:test")
	t.Setenv("WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_LOCAL_MODE", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	t.Setenv("TELEGRAM_INIT_TIMEOUT", "")

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "WEBHOOK_URL is required") {
		t.Fatalf("run = %v, want the WEBHOOK_URL error", err)
	}
}
