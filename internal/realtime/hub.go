// Package realtime pushes "something changed elsewhere" events to open browser
// sessions.
//
// It carries event names, not rendered content. A tab may be on any page, so
// the server cannot know what to swap; instead the client is told what changed
// and refetches through the ordinary handlers. That keeps one rendering path
// for both the initial load and the update.
//
// The hub is in-process. Events therefore reach only the sessions attached to
// this instance — fine for a single deployment, and the reason the domain talks
// to an interface rather than to this package: a Postgres LISTEN/NOTIFY
// implementation can replace it without the domain noticing.
package realtime

import (
	"context"
	"log/slog"
	"sync"

	"makeMe/internal/domain/task"
)

// Event names sent to the browser.
const (
	// EventProgress means this user's learning state moved — usually because
	// they answered in the other channel.
	EventProgress = "progress"
	// EventLeaderboard means a group's standings changed.
	EventLeaderboard = "leaderboard"
)

// client is one open socket.
type client struct {
	userID  int64
	groupID int64
	// send is buffered; a client that cannot keep up is dropped rather than
	// allowed to block the publisher. These messages are hints to refetch, so
	// losing one costs nothing — the next one, or the next page load, corrects it.
	send chan string
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func New() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// sendBuffer is small on purpose: these are coalescible hints, not a stream.
const sendBuffer = 8

func (h *Hub) add(userID, groupID int64) *client {
	c := &client{userID: userID, groupID: groupID, send: make(chan string, sendBuffer)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// Connections reports how many sockets are attached. Used by tests and by the
// health of a deployment; not part of the domain contract.
func (h *Hub) Connections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// TaskAnswered implements task.Notifier.
//
// The user's own sessions get progress, and — when the answer scored — also a
// leaderboard hint, because their own standings tab must refresh too. Everyone
// else in the group gets only the leaderboard hint; another member's progress
// is none of their business.
//
// The tab that did the answering also receives progress: the hub cannot tell
// sessions apart, and adding a socket identity to the domain event would drag
// transport concerns into it. The client suppresses the note for a moment
// after its own request instead — see static/js/app.js.
func (h *Hub) TaskAnswered(_ context.Context, ev task.AnswerEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	scoredInGroup := ev.Scored && ev.GroupID != 0

	for c := range h.clients {
		if c.userID == ev.UserID {
			h.deliver(c, EventProgress)
			if scoredInGroup {
				h.deliver(c, EventLeaderboard)
			}
			continue
		}
		if scoredInGroup && c.groupID == ev.GroupID {
			h.deliver(c, EventLeaderboard)
		}
	}
}

// deliver never blocks. A full buffer means the client is not reading, and a
// dropped hint is harmless.
func (h *Hub) deliver(c *client, event string) {
	select {
	case c.send <- event:
	default:
		slog.Debug("realtime: dropping event for a slow client", "user_id", c.userID, "event", event)
	}
}

var _ task.Notifier = (*Hub)(nil)
