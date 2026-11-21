package telegram_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-telegram/bot"

	"makeMe/internal/telegram"
)

// newBotAgainst builds a bot whose Bot API is srv. No database: nothing here
// handles an update.
func newBotAgainst(t *testing.T, srv *httptest.Server) *telegram.Bot {
	t.Helper()
	b, err := telegram.New("test-token", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		bot.WithSkipGetMe(), bot.WithServerURL(srv.URL))
	if err != nil {
		t.Fatalf("telegram.New: %v", err)
	}
	return b
}

// The secret is only half of the contract: /webhook checks the header, but
// Telegram sends it only if setWebhook carried secret_token. Losing it here
// would turn every real update into a 401.
func TestSetWebhook_SendsSecretToken(t *testing.T) {
	calls := make(chan url.Values, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/setWebhook") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse setWebhook body: %v", err)
			}
			calls <- r.MultipartForm.Value
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":true}`)
	}))
	defer srv.Close()

	b := newBotAgainst(t, srv)
	if err := b.SetWebhook(context.Background(), "https://example.com/webhook", "abc_DEF-123"); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}
	got := <-calls
	if u := got.Get("url"); u != "https://example.com/webhook" {
		t.Errorf("url = %q", u)
	}
	if s := got.Get("secret_token"); s != "abc_DEF-123" {
		t.Errorf("secret_token = %q, want abc_DEF-123", s)
	}
}
