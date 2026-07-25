package web

import (
	"log/slog"
	"net/http"
	"strconv"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/user"
)

// assessSlot is one question of the run.
type assessSlot struct {
	Question *assessment.Question
	Number   int // 1-based position in the run
	Total    int
	Phase    string // "словарный запас" | "грамматика"
}

// assessResult is the closing screen: the level and how it was earned.
type assessResult struct {
	Level   user.Level
	Answers []*assessment.Answer
	Correct int
	Total   int
}

type assessPage struct {
	chrome
	Slot   *assessSlot
	Result *assessResult
}

func (s *Server) handleAssess(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	session, question, err := s.assessSvc.Start(r.Context(), u.ID)
	if err != nil {
		slog.Error("web: start assessment", "user_id", u.ID, "error", err)
		s.renderError(w, "Не удалось начать тест. Попробуйте позже.")
		return
	}

	answered, err := s.assessSvc.AnswerCount(r.Context(), session.ID)
	if err != nil {
		slog.Error("web: count answers", "user_id", u.ID, "error", err)
		answered = 0
	}

	s.render(w, "assess.html", assessPage{
		chrome: chrome{User: u},
		Slot:   s.buildAssessSlot(question, answered),
	})
}

func (s *Server) handleAssessAnswer(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	// "Не знаю" posts the sentinel rather than an option index; it is recorded
	// as an honest skip and counts as wrong for level adjustment.
	option := assessment.SkipOption
	if raw := r.FormValue("option"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "bad option", http.StatusBadRequest)
			return
		}
		option = parsed
	}

	session, next, answers, err := s.assessSvc.SubmitAnswer(r.Context(), u.ID, option)
	if err != nil {
		slog.Error("web: submit assessment answer", "user_id", u.ID, "error", err)
		// Most likely the run was finished in the other channel between the
		// page render and this click. Reload rather than show a failure.
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if next == nil {
		correct := 0
		for _, a := range answers {
			if a.Correct {
				correct++
			}
		}
		slog.Info("web: assessment completed", "user_id", u.ID, "level", session.DeterminedLevel)
		s.render(w, "assess_result", assessResult{
			Level:   session.DeterminedLevel,
			Answers: answers,
			Correct: correct,
			Total:   len(answers),
		})
		return
	}

	answered, err := s.assessSvc.AnswerCount(r.Context(), session.ID)
	if err != nil {
		answered = 0
	}
	s.render(w, "assess_slot", s.buildAssessSlot(next, answered))
}

func (s *Server) buildAssessSlot(q *assessment.Question, answered int) *assessSlot {
	phase := "словарный запас"
	if answered >= s.assessSvc.VocabPhaseCount() {
		phase = "грамматика"
	}
	return &assessSlot{
		Question: q,
		Number:   answered + 1,
		Total:    s.assessSvc.TotalQuestions(),
		Phase:    phase,
	}
}
