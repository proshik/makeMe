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
