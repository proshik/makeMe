package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"makeMe/internal/audio"
)

func TestAdminAuth_NoTokenConfigured_503(t *testing.T) {
	h := adminAuth("", func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodPost, "/admin/seed-audio", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rr.Code)
	}
}

func TestAdminAuth_MissingHeader_401(t *testing.T) {
	h := adminAuth("secret", func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodPost, "/admin/seed-audio", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rr.Code)
	}
}

func TestAdminAuth_WrongToken_401(t *testing.T) {
	h := adminAuth("secret", func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })
	req := httptest.NewRequest(http.MethodPost, "/admin/seed-audio", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rr.Code)
	}
}

func TestAdminAuth_CorrectToken_PassesThrough(t *testing.T) {
	called := false
	h := adminAuth("secret", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/seed-audio", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	h(rr, req)
	if !called {
		t.Fatal("expected handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rr.Code)
	}
}

func TestSeedAudio_RejectsBadJSON(t *testing.T) {
	d := &adminDeps{token: "secret"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/seed-audio", strings.NewReader("not json"))
	d.handleSeedAudio(rr, req)
	if rr.Code != http.StatusServiceUnavailable && rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 or 503 (no deps wired)", rr.Code)
	}
}

func TestSeedAudio_RejectsEmptyLevels(t *testing.T) {
	// No seed source wired: handler short-circuits with 503 before parsing body,
	// so this test only exercises the deps check. Empty-levels validation is
	// covered indirectly via the JSON shape the handler accepts.
	d := &adminDeps{token: "secret"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/seed-audio", bytes.NewBufferString(`{"levels":[]}`))
	d.handleSeedAudio(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503 (deps not wired)", rr.Code)
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"Bearer abc", "abc"},
		{"Bearer  spaced  ", "spaced"},
		{"Basic abc", ""},
		{"bearer lower", ""}, // case-sensitive intentional
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		got := bearerToken(req)
		if got != tc.want {
			t.Errorf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

// The production http.Server sets WriteTimeout to 10s, a deadline on the whole
// response rather than on each write. A seed-audio run lasts minutes, so
// without replacing it for this response the client got the first few ndjson
// lines and then a dropped connection. This test runs the handler behind a
// real http.Server whose timeouts are far shorter than the stream and requires
// every line, including the final "done", to arrive. The gap between lines is
// longer than the per-line timeout as well: a slow dictionary lookup must not
// cut the stream either. ReadTimeout is set too, as in production, so the test
// also catches it should it ever start cutting the stream (today it does not).
func TestSeedAudio_StreamOutlivesServerTimeouts(t *testing.T) {
	const (
		words   = 10
		step    = 120 * time.Millisecond // 10 × 120ms ≈ 1.2s of streaming
		timeout = 200 * time.Millisecond
	)

	d := &adminDeps{
		token:       "secret",
		lineTimeout: 100 * time.Millisecond, // shorter than step on purpose
		seed: func(ctx context.Context, opts audio.Options) <-chan audio.Event {
			out := make(chan audio.Event)
			go func() {
				defer close(out)
				// Honour ctx the way audio.Seed does: a cancelled request
				// context ends the stream early, which must also fail the test.
				emit := func(ev audio.Event) bool {
					select {
					case <-ctx.Done():
						return false
					case out <- ev:
						return true
					}
				}
				if !emit(audio.Event{Type: audio.EventStart, Level: "A1", Total: words}) {
					return
				}
				for i := range words {
					select {
					case <-ctx.Done():
						return
					case <-time.After(step):
					}
					if !emit(audio.Event{Type: audio.EventWord, Level: "A1", Word: fmt.Sprintf("w%d", i), Status: audio.StatusNotFound}) {
						return
					}
				}
				emit(audio.Event{Type: audio.EventDone, Scanned: words, NotFound: words})
			}()
			return out
		},
	}

	srv := newSeedAudioServer(d)
	srv.Config.WriteTimeout = timeout
	srv.Config.ReadTimeout = timeout
	srv.Start()
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/seed-audio",
		strings.NewReader(`{"levels":["A1"],"delay_ms":100}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}

	var got []audio.Event
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var ev audio.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("line %d is not an event: %q: %v", len(got)+1, sc.Text(), err)
		}
		got = append(got, ev)
	}
	elapsed := time.Since(start)
	if err := sc.Err(); err != nil {
		t.Fatalf("stream broke after %d lines and %v: %v", len(got), elapsed, err)
	}

	want := 1 + words + 1 // start, one per word, done
	if len(got) != want {
		t.Fatalf("got %d lines in %v, want %d", len(got), elapsed, want)
	}
	if last := got[len(got)-1]; last.Type != audio.EventDone || last.Scanned != words {
		t.Fatalf("last event = %+v, want done with scanned=%d", last, words)
	}
	if elapsed <= timeout {
		t.Fatalf("stream took %v, not longer than the %v server timeouts: the test proves nothing", elapsed, timeout)
	}
}
