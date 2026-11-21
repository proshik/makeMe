package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	forms   []url.Values // multipart fields of each call, same order as methods
}

func (a *recordingBotAPI) called() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.methods...)
}

func (a *recordingBotAPI) form(i int) url.Values {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.forms[i]
}

func newRecordingBot(t *testing.T) (*telegram.Bot, *recordingBotAPI) {
	t.Helper()
	api := &recordingBotAPI{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var form url.Values
		if err := r.ParseMultipartForm(1 << 20); err == nil {
			form = r.MultipartForm.Value
		}
		api.mu.Lock()
		api.methods = append(api.methods, r.URL.Path)
		api.forms = append(api.forms, form)
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
	// The secret must reach Telegram from the config: without it Telegram sends
	// no header and the router answers every real update with 401.
	form := api.form(0)
	if got := form.Get("url"); got != cfg.WebhookURL {
		t.Errorf("url = %q, want %q", got, cfg.WebhookURL)
	}
	if got := form.Get("secret_token"); got != cfg.TelegramWebhookSecret {
		t.Errorf("secret_token = %q, want %q", got, cfg.TelegramWebhookSecret)
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

// serve must not touch the webhook, nor start the scheduler, if the port is
// taken: the task is about to die, and the one still serving keeps its
// registration.
func TestServe_PortTakenLeavesWebhookAlone(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()

	b, api := newRecordingBot(t)
	cfg := &config.Config{WebhookURL: "https://example.com/webhook", TelegramWebhookSecret: "abc"}
	srv := &http.Server{Addr: taken.Addr().String(), Handler: http.NotFoundHandler()}
	started := false
	err = serve(context.Background(), cfg, srv, b, func() { started = true })
	if err == nil || !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("serve = %v, want a listen error", err)
	}
	if got := api.called(); len(got) != 0 {
		t.Fatalf("Bot API was called with the port taken: %v", got)
	}
	if started {
		t.Fatal("workers started although startup failed")
	}
}

// On the way up: port bound, then setWebhook, then workers, then serving; and
// a failed setWebhook stops startup before the workers.
func TestServe_RegistersBeforeWorkersAndStopsOnFailure(t *testing.T) {
	b, api := newRecordingBot(t)
	cfg := &config.Config{WebhookURL: "https://example.com/webhook", TelegramWebhookSecret: "abc"}
	srv := &http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan []string, 1)
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, cfg, srv, b, func() { started <- api.called() })
	}()
	select {
	case calls := <-started:
		if len(calls) != 1 || calls[0] != "/bottest-token/setWebhook" {
			t.Fatalf("at worker start the Bot API had seen %v, want exactly setWebhook", calls)
		}
	case err := <-done:
		t.Fatalf("serve returned early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("workers never started")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve after shutdown = %v, want nil", err)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: bad webhook"}`)
	}))
	defer failing.Close()
	fb, err := telegram.New("test-token", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		bot.WithSkipGetMe(), bot.WithServerURL(failing.URL))
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	err = serve(context.Background(), cfg, &http.Server{Addr: "127.0.0.1:0"}, fb, func() { ran = true })
	if err == nil || !strings.Contains(err.Error(), "set webhook") {
		t.Fatalf("serve = %v, want the setWebhook error", err)
	}
	if ran {
		t.Fatal("workers started although setWebhook failed")
	}
}
