package web

import (
	"net/http"
)

// handleWS upgrades an authenticated request to a WebSocket.
//
// The connection is registered with the user's first group so leaderboard
// events can be routed; a user in no group still gets their own progress
// events.
//
// No deadline handling is needed here, contrary to the usual warning about
// http.Server's WriteTimeout killing long-lived connections. That warning does
// not apply to a hijacked connection: net/http clears both deadlines inside
// hijackLocked (rwc.SetDeadline(time.Time{})), and websocket.Accept hijacks.
// Verified by running against a server with a WriteTimeout well below the
// socket's lifetime — see TestSocketOutlivesWriteTimeout.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		http.Error(w, "realtime disabled", http.StatusNotFound)
		return
	}
	u := UserFromContext(r.Context())

	var groupID int64
	if groups, err := s.groupSvc.ListByUser(r.Context(), u.ID); err == nil && len(groups) > 0 {
		groupID = groups[0].ID
	}

	s.hub.Serve(w, r, u.ID, groupID)
}
