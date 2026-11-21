package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
