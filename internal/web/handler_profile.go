package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/words"
)

type profilePage struct {
	chrome

	MonthlyScore float64
	Streak       int

	// Vocabulary, split by how a word got there: the SRS count is words being
	// rehearsed long-term, the /words counts are the active drill.
	InSpacedRepetition int
	LearnedViaWords    int
	InProgress         int
	ByLevel            []words.LevelProgress

	Groups []group.Group
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	ctx := r.Context()

	page := profilePage{chrome: chrome{User: u, Section: SectionAccount}}

	now := time.Now()
	if score, err := s.scoringSvc.GetMonthlyScore(ctx, u.ID, now.Format("2006-01")); err == nil {
		page.MonthlyScore = score
	} else {
		slog.Error("web: monthly score", "user_id", u.ID, "error", err)
	}

	if days, err := s.scoringSvc.GetScoreDays(ctx, u.ID); err == nil {
		page.Streak = scoring.Streak(days, now)
	} else {
		slog.Error("web: score days", "user_id", u.ID, "error", err)
	}

	if n, err := s.srsRepo.CountLearnedWords(ctx, u.ID); err == nil {
		page.InSpacedRepetition = n
	}
	if n, err := s.wordsSvc.CountLearnedViaWords(ctx, u.ID); err == nil {
		page.LearnedViaWords = n
	}
	if n, err := s.wordsSvc.CountInProgress(ctx, u.ID); err == nil {
		page.InProgress = n
	}
	if levels, err := s.wordsSvc.LevelProgress(ctx, u.ID); err == nil {
		page.ByLevel = levels
	}

	if groups, err := s.groupSvc.ListByUser(ctx, u.ID); err == nil {
		page.Groups = groups
	}

	s.render(w, "profile.html", page)
}

// --- leaderboard ---------------------------------------------------------

type leaderboardPage struct {
	chrome
	Groups  []group.Group
	Group   *group.Group
	Period  string // "day" | "month"
	Entries []scoring.LeaderboardEntry
	MeID    int64
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	page, ok := s.leaderboardData(r)
	if !ok {
		s.renderError(w, "Не удалось загрузить рейтинг. Попробуйте позже.")
		return
	}
	s.render(w, "leaderboard.html", page)
}

// leaderboardData assembles the page for both the full render and the fragment
// the socket refreshes, so the two can never disagree.
func (s *Server) leaderboardData(r *http.Request) (leaderboardPage, bool) {
	u := UserFromContext(r.Context())
	ctx := r.Context()

	groups, err := s.groupSvc.ListByUser(ctx, u.ID)
	if err != nil {
		slog.Error("web: list groups", "user_id", u.ID, "error", err)
		return leaderboardPage{}, false
	}

	page := leaderboardPage{chrome: chrome{User: u, Section: SectionLeaderboard},
		Groups: groups, MeID: u.ID, Period: "day"}
	if r.URL.Query().Get("period") == "month" {
		page.Period = "month"
	}
	if len(groups) == 0 {
		return page, true
	}

	// Default to the first group; an explicit ?group= wins, but only if the
	// user is actually in it — otherwise any id would expose another group's
	// standings.
	selected := groups[0]
	if raw := r.URL.Query().Get("group"); raw != "" {
		for _, g := range groups {
			if raw == strconv.FormatInt(g.ID, 10) {
				selected = g
				break
			}
		}
	}
	page.Group = &selected

	periodKey := time.Now().Format("2006-01-02")
	if page.Period == "month" {
		periodKey = time.Now().Format("2006-01")
	}

	entries, err := s.scoringSvc.GetLeaderboard(ctx, selected.ID, page.Period, periodKey)
	if err != nil {
		slog.Error("web: leaderboard", "group_id", selected.ID, "error", err)
		return leaderboardPage{}, false
	}
	page.Entries = entries

	return page, true
}

// handleLeaderboardTable re-renders just the standings. It is what the socket
// event triggers, so a live update travels the same code path as a page load.
func (s *Server) handleLeaderboardTable(w http.ResponseWriter, r *http.Request) {
	page, ok := s.leaderboardData(r)
	if !ok || page.Group == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.render(w, "leaderboard_table", page)
}

// --- groups --------------------------------------------------------------

type groupsPage struct {
	chrome
	Groups  []group.Group
	Error   string
	Created *group.Group
	Joined  *group.Group
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	s.renderGroups(w, r, groupsPage{})
}

func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.renderGroups(w, r, groupsPage{Error: "Введите название группы."})
		return
	}
	if len([]rune(name)) > 60 {
		s.renderGroups(w, r, groupsPage{Error: "Название длиннее 60 символов — сократите."})
		return
	}

	g, err := s.groupSvc.Create(r.Context(), name, u.ID)
	if err != nil {
		slog.Error("web: create group", "user_id", u.ID, "error", err)
		s.renderGroups(w, r, groupsPage{Error: "Не удалось создать группу. Попробуйте позже."})
		return
	}
	slog.Info("web: group created", "user_id", u.ID, "group_id", g.ID)
	s.renderGroups(w, r, groupsPage{Created: g})
}

func (s *Server) handleGroupJoin(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		s.renderGroups(w, r, groupsPage{Error: "Введите инвайт-код."})
		return
	}

	g, err := s.groupSvc.Join(r.Context(), code, u.ID)
	switch {
	case errors.Is(err, group.ErrAlreadyMember):
		s.renderGroups(w, r, groupsPage{Error: "Вы уже состоите в этой группе."})
		return
	case err != nil:
		// A wrong code and a missing group are the same thing to the user, and
		// distinguishing them would let someone probe which codes exist.
		slog.Info("web: join failed", "user_id", u.ID, "error", err)
		s.renderGroups(w, r, groupsPage{Error: "Группа по такому коду не найдена. Проверьте код."})
		return
	}

	slog.Info("web: group joined", "user_id", u.ID, "group_id", g.ID)
	s.renderGroups(w, r, groupsPage{Joined: g})
}

func (s *Server) renderGroups(w http.ResponseWriter, r *http.Request, page groupsPage) {
	u := UserFromContext(r.Context())
	page.chrome = chrome{User: u, Section: SectionAccount}
	if groups, err := s.groupSvc.ListByUser(r.Context(), u.ID); err == nil {
		page.Groups = groups
	} else {
		slog.Error("web: list groups", "user_id", u.ID, "error", err)
	}
	s.render(w, "groups.html", page)
}
