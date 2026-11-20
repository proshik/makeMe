package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
