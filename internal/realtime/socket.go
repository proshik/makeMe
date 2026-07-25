package realtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	// pingEvery keeps intermediaries from reaping an idle connection. Proxies
	// commonly cut silent sockets after a minute.
	pingEvery = 25 * time.Second
	// writeWait bounds a single write so one wedged client cannot pin the
	// goroutine forever.
	writeWait = 10 * time.Second
)

// Serve upgrades the request and pumps events until the client goes away or the
// server shuts down.
//
// The caller is responsible for authentication — this package never inspects
// cookies. groupID may be 0 for a user who is in no group; such a client simply
// never matches a leaderboard event.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID, groupID int64) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Same-origin only. Without this the browser would happily let another
		// site open an authenticated socket with the user's cookies.
		OriginPatterns: nil,
	})
	if err != nil {
		slog.Debug("realtime: upgrade failed", "error", err)
		return
	}
	defer conn.CloseNow()

	c := h.add(userID, groupID)
	defer h.remove(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// The client sends nothing; reading exists only to observe the close frame
	// and to surface a dead connection promptly.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()

	slog.Debug("realtime: client connected", "user_id", userID, "group_id", groupID)

	ticker := time.NewTicker(pingEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-c.send:
			if !ok {
				return
			}
			if err := writeEvent(ctx, conn, event); err != nil {
				if !errors.Is(err, context.Canceled) {
					slog.Debug("realtime: write failed", "user_id", userID, "error", err)
				}
				return
			}

		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, writeWait)
			err := conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return
			}
		}
	}
}

func writeEvent(ctx context.Context, conn *websocket.Conn, event string) error {
	ctx, cancel := context.WithTimeout(ctx, writeWait)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, []byte(event))
}
