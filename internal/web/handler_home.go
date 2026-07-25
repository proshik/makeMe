package web

import (
	"net/http"
)

type homePage struct {
	chrome
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, "home.html", homePage{chrome: chrome{User: UserFromContext(r.Context())}})
}

// handleNotFound renders a 404 inside the site chrome. It is deliberately
// public: sending an anonymous visitor to /login for a mistyped URL would be
// more confusing than telling them the page does not exist.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	// UserFromContext is nil for anonymous visitors; the nav renders a "Войти"
	// link instead of the account menu in that case.
	w.WriteHeader(http.StatusNotFound)
	s.render(w, "notfound.html", chrome{User: UserFromContext(r.Context())})
}
