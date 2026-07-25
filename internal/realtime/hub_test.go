package realtime

import (
	"context"
	"testing"

	"makeMe/internal/domain/task"
)

func drain(c *client) []string {
	var got []string
	for {
		select {
		case ev := <-c.send:
			got = append(got, ev)
		default:
			return got
		}
	}
}

// The user's own other sessions are the point: answering in the bot should
// reach the browser tab they left open. A wrong answer still moves their
// progress, so it is delivered even though the standings cannot have changed.
func TestOwnSessionsGetProgress(t *testing.T) {
	h := New()
	a := h.add(1, 10)
	b := h.add(1, 10)

	h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 1, GroupID: 10, Scored: false})

	for i, c := range []*client{a, b} {
		got := drain(c)
		if len(got) != 1 || got[0] != EventProgress {
			t.Fatalf("client %d got %v, want one %q", i, got, EventProgress)
		}
	}
}

// Group members hear that the standings moved — but only when the answer
// actually scored, since a wrong answer cannot change them.
func TestGroupGetsLeaderboardOnlyWhenScored(t *testing.T) {
	h := New()
	other := h.add(2, 10)

	h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 1, GroupID: 10, Scored: false})
	if got := drain(other); len(got) != 0 {
		t.Fatalf("a wrong answer must not move the leaderboard, got %v", got)
	}

	h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 1, GroupID: 10, Scored: true})
	if got := drain(other); len(got) != 1 || got[0] != EventLeaderboard {
		t.Fatalf("got %v, want one %q", got, EventLeaderboard)
	}
}

// The user's own leaderboard tab must refresh when they score, or their own
// standings would sit stale while everyone else's updated.
func TestOwnSessionAlsoGetsLeaderboardWhenScored(t *testing.T) {
	h := New()
	mine := h.add(1, 10)

	h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 1, GroupID: 10, Scored: true})

	got := drain(mine)
	if len(got) != 2 {
		t.Fatalf("got %v, want progress and leaderboard", got)
	}
	if got[0] != EventProgress || got[1] != EventLeaderboard {
		t.Fatalf("got %v, want [%s %s]", got, EventProgress, EventLeaderboard)
	}
}

func TestOtherGroupsHearNothing(t *testing.T) {
	h := New()
	stranger := h.add(3, 99)

	h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 1, GroupID: 10, Scored: true})

	if got := drain(stranger); len(got) != 0 {
		t.Fatalf("a member of another group received %v", got)
	}
}

// A user with no group must still get their own progress; they simply never
// match a leaderboard event.
func TestGrouplessUserStillGetsProgress(t *testing.T) {
	h := New()
	c := h.add(4, 0)

	h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 4, GroupID: 0, Scored: false})

	if got := drain(c); len(got) != 1 || got[0] != EventProgress {
		t.Fatalf("got %v, want one %q", got, EventProgress)
	}
}

// A client that stopped reading must not block the publisher: these are hints
// to refetch, and dropping one costs nothing.
func TestSlowClientIsNotAllowedToBlock(t *testing.T) {
	h := New()
	c := h.add(5, 10)

	for i := 0; i < sendBuffer*4; i++ {
		h.TaskAnswered(context.Background(), task.AnswerEvent{UserID: 5, Scored: false})
	}

	if got := len(drain(c)); got != sendBuffer {
		t.Fatalf("buffered %d events, want the buffer to cap at %d", got, sendBuffer)
	}
}

func TestRemoveClosesAndDeregisters(t *testing.T) {
	h := New()
	c := h.add(6, 10)
	if h.Connections() != 1 {
		t.Fatalf("Connections = %d, want 1", h.Connections())
	}

	h.remove(c)
	if h.Connections() != 0 {
		t.Fatalf("Connections = %d, want 0", h.Connections())
	}
	if _, open := <-c.send; open {
		t.Fatal("send channel should be closed so the pump exits")
	}

	// Removing twice must not panic on a double close.
	h.remove(c)
}

// The service must work with no notifier attached — that is the bot-only
// deployment and every test that does not care about realtime.
func TestNilNotifierIsSafe(t *testing.T) {
	var n task.Notifier
	if n != nil {
		t.Fatal("expected a nil interface")
	}
}
