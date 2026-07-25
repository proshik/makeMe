package web

import (
	"net/http"

	"makeMe/internal/domain/user"
)

type homePage struct {
	User *user.User
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, "home.html", homePage{User: UserFromContext(r.Context())})
}

// handleNotFound renders a 404 inside the site chrome. It is deliberately
// public: sending an anonymous visitor to /login for a mistyped URL would be
// more confusing than telling them the page does not exist.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	s.render(w, "notfound.html", nil)
}
