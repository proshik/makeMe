package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"makeMe/internal/domain/words"
)

// Pauses before /words moves on by itself after an answer.
const (
	autoAdvanceCorrectMS = 1000
	autoAdvanceWrongMS   = 3000
)

// wordSlot is one drill of one word in one mode.
type wordSlot struct {
	SessionID int64
	WordID    int64
	Mode      words.Mode
	Typing    bool

	Prompt      string // the side being asked: English for EN→RU, Russian for RU→EN
	Transcript  string // IPA, only meaningful when the English side is shown
	Example     string
	AudioText   string // English word to speak; empty in RU→EN modes, where it is the answer
	AudioURL    string
	Instruction string
	Options     []string

	Shown  int // how many drills are done in this session
	Total  int // how many drills the session needs in all
	Counts []modeCount
}

// modeCount is the per-mode tally rendered as flaps: two correct answers each.
type modeCount struct {
	Label string
	Done  int
	Need  int
}

type wordsPage struct {
	chrome
	Slot       *wordSlot
	Status     *words.StatusView
	Done       bool
	NeedsLevel bool
}

// wordFeedback is the fragment shown after an answer.
type wordFeedback struct {
	SessionID  int64
	WordID     int64
	Correct    bool
	Skipped    bool
	Expected   string
	Learned    bool // the word completed all four modes just now
	OfferDefer bool // too many mistakes on this word in this session
	English    string
	Russian    string
	Transcript string
	AudioText  string
	AudioURL   string
	// Next is the following drill, rendered inert into the feedback so
	// "Дальше" can show it without a second round trip. Nil when the session
	// is exhausted (the hx-get fallback then closes it) and when the card
	// offers to defer the word — deferring changes what comes next.
	Next *wordSlot
	// AutoAdvanceMS moves on to the next word by itself: a short beat after a
	// correct answer, long enough to read the right one after a mistake. Zero
	// when the card asks for a decision (defer the word) or announces that the
	// word is learned — those wait for the button.
	AutoAdvanceMS int
}

func (s *Server) handleWords(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	if !u.Onboarded {
		s.render(w, "words.html", wordsPage{chrome: wordsChrome(u), NeedsLevel: true})
		return
	}

	session, _, _, err := s.wordsSvc.StartOrResumeSession(r.Context(), u.ID)
	if err != nil {
		slog.Error("web: start words session", "user_id", u.ID, "error", err)
		s.renderError(w, "Не удалось загрузить слова. Попробуйте позже.")
		return
	}

	status, err := s.wordsSvc.Status(r.Context(), u.ID, session)
	if err != nil {
		slog.Warn("web: words status", "user_id", u.ID, "error", err)
	}

	slot, done, err := s.buildWordSlot(r, session.ID, &status)
	if err != nil {
		slog.Error("web: next word slot", "user_id", u.ID, "error", err)
		s.renderError(w, "Не удалось загрузить слово. Попробуйте позже.")
		return
	}

	s.render(w, "words.html", wordsPage{chrome: wordsChrome(u), Slot: slot, Status: &status, Done: done})
}

// handleWordsNext renders the next slot as a fragment.
func (s *Server) handleWordsNext(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	session, err := s.wordsSvc.RepoOpenSession(r.Context(), u.ID)
	if err != nil || session == nil {
		s.render(w, "words_done", nil)
		return
	}

	status, _ := s.wordsSvc.Status(r.Context(), u.ID, session)
	slot, done, err := s.buildWordSlot(r, session.ID, &status)
	if err != nil {
		slog.Error("web: next word slot", "user_id", u.ID, "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	if done {
		s.render(w, "words_done", nil)
		return
	}
	s.render(w, "word_slot", slot)
}

// buildWordSlot assembles the next drill, including its multiple-choice options
// when the mode calls for them.
func (s *Server) buildWordSlot(r *http.Request, sessionID int64, status *words.StatusView) (*wordSlot, bool, error) {
	slot, session, done, err := s.wordsSvc.NextSlot(r.Context(), sessionID)
	if err != nil {
		return nil, false, err
	}
	if done {
		// Finishing the drill closes the session, so the next /words visit
		// starts a fresh pick rather than resuming an exhausted one.
		if err := s.wordsSvc.CompleteSession(r.Context(), sessionID); err != nil {
			slog.Warn("web: complete words session", "session_id", sessionID, "error", err)
		}
		return nil, true, nil
	}
	return s.wordSlotView(r, slot, session, status)
}

// peekWordSlot builds the drill that would come next without writing
// anything: when the session is exhausted it returns nil and leaves the
// session open, so closing still happens on the user's own "Дальше".
func (s *Server) peekWordSlot(r *http.Request, sessionID int64) (*wordSlot, error) {
	slot, session, done, err := s.wordsSvc.PeekNextSlot(r.Context(), sessionID)
	if err != nil || done {
		return nil, err
	}
	status, err := s.wordsSvc.Status(r.Context(), session.UserID, session)
	if err != nil {
		return nil, err
	}
	view, _, err := s.wordSlotView(r, slot, session, &status)
	return view, err
}

func (s *Server) wordSlotView(r *http.Request, slot *words.Slot, session *words.Session, status *words.StatusView) (*wordSlot, bool, error) {
	word := slot.Word
	view := &wordSlot{
		SessionID: session.ID,
		WordID:    word.ID,
		Mode:      slot.Mode,
		Typing:    slot.IsTyping(),
		Shown:     status.ModesCompleted,
		Total:     status.ModesTotal,
		Counts:    s.modeCounts(r, session.UserID, word.ID),
	}

	switch slot.Mode {
	case words.ModeEnRuMC:
		view.Prompt, view.Transcript, view.Example = word.English, word.Transcription, word.Example
		view.AudioText, view.AudioURL = word.English, word.AudioURL
		view.Instruction = "Выберите перевод на русский"
	case words.ModeRuEnMC:
		view.Prompt = word.Russian
		view.Instruction = "Выберите перевод на английский"
	case words.ModeEnRuTyping:
		view.Prompt, view.Transcript = word.English, word.Transcription
		view.AudioText, view.AudioURL = word.English, word.AudioURL
		view.Instruction = "Введите перевод на русский"
	case words.ModeRuEnTyping:
		view.Prompt = word.Russian
		view.Instruction = "Введите перевод на английский"
	}

	if slot.IsMC() {
		distractors, err := s.srsRepo.GetWordsByLevel(r.Context(), word.Level, 10)
		if err != nil {
			return nil, false, err
		}
		// Options are built by the domain so both channels drill from the same
		// pool in the same way. The correct index is not sent to the browser —
		// correctness is decided server-side from the submitted text.
		view.Options, _ = words.BuildMCOptions(words.ExpectedAnswer(word, slot.Mode), distractors, slot.Mode)
	}

	return view, false, nil
}

func (s *Server) modeCounts(r *http.Request, userID, wordID int64) []modeCount {
	progress, err := s.wordsSvc.RepoGetProgress(r.Context(), userID, wordID)
	if err != nil || progress == nil {
		progress = &words.Progress{Counts: map[words.Mode]int{}}
	}
	labels := map[words.Mode]string{
		words.ModeEnRuMC:     "выбор EN→RU",
		words.ModeRuEnMC:     "выбор RU→EN",
		words.ModeEnRuTyping: "ввод EN→RU",
		words.ModeRuEnTyping: "ввод RU→EN",
	}
	out := make([]modeCount, 0, len(words.AllModes))
	for _, m := range words.AllModes {
		done := progress.Counts[m]
		if done > words.ThresholdPerMode {
			done = words.ThresholdPerMode
		}
		out = append(out, modeCount{Label: labels[m], Done: done, Need: words.ThresholdPerMode})
	}
	return out
}

func (s *Server) handleWordsAnswer(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	wordID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad word id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	session, err := s.wordsSvc.RepoOpenSession(r.Context(), u.ID)
	if err != nil || session == nil {
		// The session was finished elsewhere — reload instead of failing.
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	mode := words.Mode(r.FormValue("mode"))
	answer := r.FormValue("answer")

	var out words.Outcome
	switch {
	case r.FormValue("skip") != "":
		out, err = s.wordsSvc.SubmitSkip(r.Context(), session, wordID, mode)
	case mode == words.ModeEnRuTyping || mode == words.ModeRuEnTyping:
		out, err = s.wordsSvc.SubmitTyping(r.Context(), session, wordID, mode, answer)
	default:
		// MC correctness is decided here, from the submitted text against the
		// expected answer — never from an index the browser sends back.
		word, wErr := s.srsRepo.GetWordByID(r.Context(), wordID)
		if wErr != nil {
			slog.Error("web: load word", "word_id", wordID, "error", wErr)
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		out, err = s.wordsSvc.SubmitMC(r.Context(), session, wordID, mode,
			answer == words.ExpectedAnswer(word, mode))
	}

	if errors.Is(err, words.ErrWordNotInSession) {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		slog.Error("web: submit word answer", "user_id", u.ID, "word_id", wordID, "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}

	s.renderWordFeedback(w, r, session.ID, wordID, out, r.FormValue("skip") != "")
}

// handleWordsKnow retires a word the user already knows without drilling it
// through all four modes — the fast path for a B2 speaker meeting A1 words.
func (s *Server) handleWordsKnow(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	wordID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad word id", http.StatusBadRequest)
		return
	}

	session, err := s.wordsSvc.RepoOpenSession(r.Context(), u.ID)
	if err != nil || session == nil {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.wordsSvc.MarkAlreadyKnown(r.Context(), session, wordID); err != nil {
		slog.Error("web: mark word known", "user_id", u.ID, "word_id", wordID, "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	slog.Info("web: word marked known", "user_id", u.ID, "word_id", wordID)

	s.handleWordsNext(w, r)
}

// handleWordsDefer drops a word from the rest of this session, keeping its
// progress. It reappears in the next one.
func (s *Server) handleWordsDefer(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	wordID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad word id", http.StatusBadRequest)
		return
	}

	session, err := s.wordsSvc.RepoOpenSession(r.Context(), u.ID)
	if err != nil || session == nil {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.wordsSvc.DeferWord(r.Context(), session, wordID); err != nil {
		slog.Error("web: defer word", "user_id", u.ID, "word_id", wordID, "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	slog.Info("web: word deferred", "user_id", u.ID, "word_id", wordID)

	s.handleWordsNext(w, r)
}

func (s *Server) renderWordFeedback(w http.ResponseWriter, r *http.Request, sessionID, wordID int64, out words.Outcome, skipped bool) {
	fb := wordFeedback{
		SessionID:  sessionID,
		WordID:     wordID,
		Correct:    out.Correct,
		Skipped:    skipped,
		Expected:   out.Expected,
		Learned:    out.BecameLearned,
		OfferDefer: !out.Correct && out.ErrorsInSession > words.DeferThreshold,
	}
	if word, err := s.srsRepo.GetWordByID(r.Context(), wordID); err == nil {
		fb.English, fb.Russian = word.English, word.Russian
		fb.Transcript, fb.AudioText, fb.AudioURL = word.Transcription, word.English, word.AudioURL
	}
	if !fb.OfferDefer {
		next, err := s.peekWordSlot(r, sessionID)
		if err != nil {
			// Not fatal: without the prefetched slot "Дальше" fetches it.
			slog.Warn("web: prefetch next word", "session_id", sessionID, "error", err)
		}
		fb.Next = next
	}
	switch {
	case fb.OfferDefer || fb.Learned:
		// Wait for the button.
	case fb.Correct:
		fb.AutoAdvanceMS = autoAdvanceCorrectMS
	default:
		fb.AutoAdvanceMS = autoAdvanceWrongMS
	}
	s.render(w, "word_feedback", fb)
}
