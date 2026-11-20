package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"makeMe/internal/audio"
)

// newSeedAudioServer serves the handler behind the same middleware as
// production, on a real http.Server with a short WriteTimeout; start it with
// srv.Start after adjusting srv.Config.
func newSeedAudioServer(d *adminDeps) *httptest.Server {
	r := chi.NewRouter()
	useCommonMiddleware(r)
	r.Post("/admin/seed-audio", adminAuth(d.token, d.handleSeedAudio))
	srv := httptest.NewUnstartedServer(r)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	return srv
}

// endlessSeed streams events carrying a word of wordLen bytes until ctx is
// cancelled and reports the cancellation on the returned channel, so a test
// can tell a run that was stopped from one still pinned on a blocked write.
func endlessSeed(wordLen int) (func(context.Context, audio.Options) <-chan audio.Event, <-chan struct{}) {
	cancelled := make(chan struct{})
	big := strings.Repeat("x", wordLen)
	seed := func(ctx context.Context, _ audio.Options) <-chan audio.Event {
		out := make(chan audio.Event)
		go func() {
			defer close(out)
			defer close(cancelled)
			for {
				select {
				case <-ctx.Done():
					return
				case out <- audio.Event{Type: audio.EventWord, Word: big, Status: audio.StatusNotFound}:
				}
			}
		}()
		return out
	}
	return seed, cancelled
}

// A client that keeps the connection open but stops reading fills the socket
// buffers, and the next write blocks. With the server's WriteTimeout lifted
// for this response, only a per-write deadline can unblock it; without one
// the handler and the seeder stay pinned for as long as the peer lingers.
//
// Both line sizes matter. A 64KB line overflows the response buffer, so the
// timeout surfaces in Encode. A real line is about 70 bytes and lands in the
// buffer; the timeout then happens inside Flush, which chi's middleware
// wrapper does not report, and the run is released only because net/http
// cancels the request context on a failed connection write.
func TestSeedAudio_StalledClientReleasesRun(t *testing.T) {
	for _, wordLen := range []int{64 << 10, 16} {
		t.Run(fmt.Sprintf("word=%dB", wordLen), func(t *testing.T) {
			testStalledClientReleasesRun(t, wordLen)
		})
	}
}

func testStalledClientReleasesRun(t *testing.T, wordLen int) {
	seed, cancelled := endlessSeed(wordLen)
	d := &adminDeps{token: "secret", seed: seed, lineTimeout: 200 * time.Millisecond}

	srv := newSeedAudioServer(d)
	srv.Start()
	t.Cleanup(srv.Close)

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.(*net.TCPConn).SetReadBuffer(4 << 10)

	body := `{"levels":["A1"]}`
	fmt.Fprintf(conn, "POST /admin/seed-audio HTTP/1.1\r\nHost: x\r\nAuthorization: Bearer secret\r\n"+
		"Content-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	// From here on the client never reads.

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("seed run still pinned 5s after the client stopped reading")
	}
}

// A client that goes away mid-stream must stop the run rather than leave the
// seeder querying the dictionary for nobody.
func TestSeedAudio_ClientDisconnectCancelsRun(t *testing.T) {
	seed, cancelled := endlessSeed(16)
	d := &adminDeps{token: "secret", seed: seed}

	srv := newSeedAudioServer(d)
	srv.Start()
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/seed-audio", strings.NewReader(`{"levels":["A1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(resp.Body).ReadString('\n'); err != nil {
		t.Fatalf("first line: %v", err)
	}
	resp.Body.Close()

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("seed run still going 5s after the client disconnected")
	}
}

// syncBuffer collects log output written from the handler's goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogs points the default slog logger at a buffer for this test.
func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// A run cut short must not be logged as "completed": that line once made a
// broken production stream look like a success.
func TestSeedAudio_AbortedRunIsNotLoggedAsCompleted(t *testing.T) {
	logs := captureLogs(t)
	seed, cancelled := endlessSeed(16)
	d := &adminDeps{token: "secret", seed: seed}

	srv := newSeedAudioServer(d)
	srv.Start()
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/seed-audio", strings.NewReader(`{"levels":["A1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(resp.Body).ReadString('\n'); err != nil {
		t.Fatalf("first line: %v", err)
	}
	resp.Body.Close()
	<-cancelled

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "admin seed-audio aborted before done") {
		if time.Now().After(deadline) {
			t.Fatalf("no abort line in the log:\n%s", logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Contains(logs.String(), "admin seed-audio completed") {
		t.Fatalf("aborted run logged as completed:\n%s", logs.String())
	}
}

// A run that reaches "done" is still logged as completed.
func TestSeedAudio_FinishedRunIsLoggedAsCompleted(t *testing.T) {
	logs := captureLogs(t)
	d := &adminDeps{token: "secret", seed: func(ctx context.Context, _ audio.Options) <-chan audio.Event {
		out := make(chan audio.Event, 1)
		out <- audio.Event{Type: audio.EventDone}
		close(out)
		return out
	}}
	srv := newSeedAudioServer(d)
	srv.Start()
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/seed-audio", strings.NewReader(`{"levels":["A1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// The handler logs before it returns, and the body ends when it returns.
	if out := logs.String(); !strings.Contains(out, "admin seed-audio completed") || strings.Contains(out, "aborted") {
		t.Fatalf("finished run not logged as completed:\n%s", out)
	}
}

// Every way a run can end without "done" reaching the client must produce the
// one documented abort line, and never "completed". A 1ns line timeout makes
// the first flush fail deterministically. Behind chi that failure is not
// reported by Flush; net/http cancels the request context instead, and the
// next Encode hits the response's sticky write error.
func TestSeedAudio_FailedWriteIsLoggedAsAborted(t *testing.T) {
	cases := map[string][]audio.Event{
		// The flush of "done" itself fails: done was encoded, never delivered.
		"done line fails": {{Type: audio.EventDone}},
		// An earlier line fails; the handler leaves through the Encode error.
		"earlier line fails": {{Type: audio.EventWord, Word: "w"}, {Type: audio.EventDone}},
		// Enough lines after the failure for the sticky error to reach Encode.
		"encode fails after it": {
			{Type: audio.EventWord, Word: "w1"}, {Type: audio.EventWord, Word: "w2"},
			{Type: audio.EventWord, Word: "w3"}, {Type: audio.EventWord, Word: "w4"},
			{Type: audio.EventDone},
		},
	}
	for name, events := range cases {
		t.Run(name, func(t *testing.T) {
			logs := captureLogs(t)
			d := &adminDeps{
				token:       "secret",
				lineTimeout: time.Nanosecond,
				seed: func(context.Context, audio.Options) <-chan audio.Event {
					out := make(chan audio.Event, len(events))
					for _, ev := range events {
						out <- ev
					}
					close(out)
					return out
				},
			}
			srv := newSeedAudioServer(d)
			srv.Start()
			t.Cleanup(srv.Close)

			req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/seed-audio", strings.NewReader(`{"levels":["A1"]}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer secret")
			if resp, err := srv.Client().Do(req); err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}

			deadline := time.Now().Add(5 * time.Second)
			for !strings.Contains(logs.String(), "admin seed-audio aborted before done") {
				if time.Now().After(deadline) {
					t.Fatalf("no abort line in the log:\n%s", logs.String())
				}
				time.Sleep(10 * time.Millisecond)
			}
			if strings.Contains(logs.String(), "admin seed-audio completed") {
				t.Fatalf("failed run logged as completed:\n%s", logs.String())
			}
		})
	}
}
