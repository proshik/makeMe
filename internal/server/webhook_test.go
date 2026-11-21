package server

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

	"makeMe/internal/telegram"
)

const testWebhookSecret = "s3cret_value-123"

// fakeBotAPI stands in for api.telegram.org and records the chat of every
// sendMessage. A processed update is one that made the bot answer, so this is
// what tells "handled" from "rejected" — not the status code alone.
type fakeBotAPI struct {
	mu    sync.Mutex
	chats []string
	sent  chan string
}

func newFakeBotAPI(t *testing.T) (*fakeBotAPI, *httptest.Server) {
	t.Helper()
	f := &fakeBotAPI{sent: make(chan string, 16)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			chat := r.FormValue("chat_id")
			f.mu.Lock()
			f.chats = append(f.chats, chat)
			f.mu.Unlock()
			f.sent <- chat
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"}}}`)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeBotAPI) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.chats...)
}

// newWebhookRouter wires a real telegram.Bot behind the production router, so
// the test covers the route as server.New mounts it rather than the guard on
// its own. No database: an update without a sender skips user resolution, and
// an unknown command is answered by the default handler alone.
func newWebhookRouter(t *testing.T, secret string) (http.Handler, *fakeBotAPI) {
	t.Helper()
	api, apiSrv := newFakeBotAPI(t)
	tgBot, err := telegram.New("test-token", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		bot.WithSkipGetMe(), bot.WithServerURL(apiSrv.URL))
	if err != nil {
		t.Fatalf("telegram.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { tgBot.StartWebhook(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return New(tgBot, secret, AdminConfig{}, nil), api
}

func postUpdate(t *testing.T, h http.Handler, chatID int64, header *string) int {
	t.Helper()
	body := fmt.Sprintf(`{"update_id":%d,"message":{"message_id":1,"date":0,"chat":{"id":%d,"type":"private"},"text":"/nosuchcommand"}}`, chatID, chatID)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if header != nil {
		req.Header.Set(webhookSecretHeader, *header)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Code
}

func waitSent(t *testing.T, api *fakeBotAPI) string {
	t.Helper()
	select {
	case chat := <-api.sent:
		return chat
	case <-time.After(5 * time.Second):
		t.Fatal("bot did not answer the update within 5s")
		return ""
	}
}

func TestWebhook_CorrectSecretIsProcessed(t *testing.T) {
	h, api := newWebhookRouter(t, testWebhookSecret)
	secret := testWebhookSecret
	if code := postUpdate(t, h, 42, &secret); code != http.StatusOK {
		t.Fatalf("got %d, want 200", code)
	}
	if chat := waitSent(t, api); chat != "42" {
		t.Fatalf("bot answered chat %q, want 42", chat)
	}
}

func TestWebhook_WrongOrMissingSecretIsRejectedAndNotProcessed(t *testing.T) {
	h, api := newWebhookRouter(t, testWebhookSecret)

	wrong := "not-the-secret"
	prefix := testWebhookSecret[:len(testWebhookSecret)-1]
	empty := ""
	cases := []struct {
		name   string
		chat   int64
		header *string
	}{
		{"missing header", 101, nil},
		{"empty header", 102, &empty},
		{"wrong secret", 103, &wrong},
		{"prefix of the secret", 104, &prefix},
	}
	for _, c := range cases {
		if code := postUpdate(t, h, c.chat, c.header); code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", c.name, code)
		}
	}

	// Updates are consumed in order by a single worker, so once the valid one
	// has been answered any rejected update that slipped into the queue would
	// have been answered too. Only chat 42 may appear.
	secret := testWebhookSecret
	if code := postUpdate(t, h, 42, &secret); code != http.StatusOK {
		t.Fatalf("valid update: got %d, want 200", code)
	}
	waitSent(t, api)
	if got := api.recorded(); len(got) != 1 || got[0] != "42" {
		t.Fatalf("bot answered chats %v, want only [42]", got)
	}
}

func TestWebhook_NoSecretConfiguredClosesEndpoint(t *testing.T) {
	h, api := newWebhookRouter(t, "")
	any := "whatever"
	for _, header := range []*string{nil, &any} {
		if code := postUpdate(t, h, 7, header); code != http.StatusServiceUnavailable {
			t.Errorf("got %d, want 503", code)
		}
	}
	if got := api.recorded(); len(got) != 0 {
		t.Fatalf("bot answered %v with the endpoint closed", got)
	}
}
