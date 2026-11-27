package web

import (
	"errors"
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
)

// taskView is everything a task slot needs to render. It is deliberately flat:
// the template does no lookups of its own.
type taskView struct {
	Task        *task.Task
	Round       int
	MaxRounds   int
	Position    int // 1-based index of this task within the round
	Total       int
	Instruction string
	Typing      bool
	Word        string // headword for vocab tasks, shown at display size
	Transcript  string // IPA — only when the English side is the prompt
	Example     string // example sentence, same condition as Transcript
	// Prompt is Task.Prompt without the HTML escaping the domain applies for
	// Telegram's HTML parse mode. html/template escapes it again on output, so
	// passing Task.Prompt through showed "I&#39;ll" on the page.
	Prompt string
	// AudioText is the English word "Послушать" speaks, AudioURL the
	// recording tried first. Both stay empty when hearing the word would give
	// the answer away: RU→EN and fill-in-the-blank.
	AudioText string
	AudioURL  string
}

// tasksPage wraps a slot for the full-page render.
type tasksPage struct {
	chrome
	Slot *taskView

	// Exactly one of these is set when there is no slot to answer.
	Done       *roundSummary
	LimitHit   bool
	MaxRounds  int
	NeedsLevel bool
}

type roundSummary struct {
	Correct int
	Total   int
}

// feedbackView is the fragment swapped in after an answer.
type feedbackView struct {
	Correct     bool
	Skipped     bool
	Answer      string
	Explanation string
	Score       int
	Level       user.Level
	HasNext     bool
	Done        *roundSummary
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	if !u.Onboarded {
		s.render(w, "tasks.html", tasksPage{chrome: tasksChrome(u), NeedsLevel: true})
		return
	}

	ds, err := s.dailySetFor(r, u)
	if err != nil {
		if errors.Is(err, task.ErrDailyLimitReached) {
			s.render(w, "tasks.html", tasksPage{
				chrome: tasksChrome(u), LimitHit: true, MaxRounds: s.taskSvc.MaxRounds(),
			})
			return
		}
		slog.Error("web: generate daily set", "user_id", u.ID, "error", err)
		s.renderError(w, "Не удалось загрузить задания. Попробуйте позже.")
		return
	}

	page := tasksPage{chrome: tasksChrome(u), MaxRounds: s.taskSvc.MaxRounds()}
	slot, summary, err := s.nextSlot(r, u, ds)
	if err != nil {
		slog.Error("web: next task", "user_id", u.ID, "error", err)
		s.renderError(w, "Не удалось загрузить задание. Попробуйте позже.")
		return
	}
	page.Slot, page.Done = slot, summary

	s.render(w, "tasks.html", page)
}

// dailySetFor resolves the user's group (creating a solo one if they have
// none — same rule as the bot, otherwise a first-time web user has nowhere for
// their score to land) and generates or resumes today's set.
func (s *Server) dailySetFor(r *http.Request, u *user.User) (*task.DailySet, error) {
	groups, err := s.groupSvc.ListByUser(r.Context(), u.ID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		g, err := s.groupSvc.Create(r.Context(), "Соло", u.ID)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *g)
	}
	return s.taskSvc.GenerateDaily(r.Context(), u.ID, groups[0].ID, u.Level)
}

// currentSet returns today's open set without generating anything. Returns
// (nil, nil) when the user has no group or no set yet.
func (s *Server) currentSet(r *http.Request, u *user.User) (*task.DailySet, error) {
	groups, err := s.groupSvc.ListByUser(r.Context(), u.ID)
	if err != nil || len(groups) == 0 {
		return nil, err
	}
	return s.taskSvc.GetTodaySet(r.Context(), u.ID, groups[0].ID)
}

// nextSlot returns the next unanswered task, or a summary when the round is
// finished. Exactly one of the two is non-nil.
func (s *Server) nextSlot(r *http.Request, u *user.User, ds *task.DailySet) (*taskView, *roundSummary, error) {
	next, err := s.taskSvc.GetNextUnanswered(r.Context(), ds.ID, u.ID)
	if err != nil {
		return nil, nil, err
	}

	if next == nil {
		_, total, err := s.taskSvc.GetProgress(r.Context(), ds.ID, u.ID)
		if err != nil {
			return nil, nil, err
		}
		correct, err := s.taskSvc.GetCorrectCount(r.Context(), ds.ID, u.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &roundSummary{Correct: correct, Total: total}, nil
	}

	answered, total, err := s.taskSvc.GetProgress(r.Context(), ds.ID, u.ID)
	if err != nil {
		return nil, nil, err
	}

	return s.buildTaskView(r, next, ds.Round, answered+1, total), nil, nil
}

func (s *Server) buildTaskView(r *http.Request, t *task.Task, round, position, total int) *taskView {
	v := &taskView{
		Task:        t,
		Round:       round + 1,
		MaxRounds:   s.taskSvc.MaxRounds(),
		Position:    position,
		Total:       total,
		Instruction: taskInstruction(t),
		Typing:      isTyping(t),
		Prompt:      html.UnescapeString(t.Prompt),
	}

	// Vocab tasks carry a word id; pull the record so the card can show the
	// headword at display size.
	//
	// Which side is shown depends on the subtype, and getting it wrong is not
	// cosmetic: showing the English word for a "type it in English" task hands
	// over the answer, and the transcription hands over the pronunciation with
	// it. Fill-blank is its own case — the prompt is the blanked sentence, and
	// showing the word alongside it would defeat the exercise.
	if t.Type == task.TypeVocab && t.Metadata != nil {
		if id := metaInt64(t.Metadata["word_id"]); id > 0 {
			if word, err := s.srsRepo.GetWordByID(r.Context(), id); err == nil && word != nil {
				switch t.Metadata["subtype"] {
				case task.VocabSubtypeRuEn, task.VocabSubtypeTypingRuEn:
					v.Word = word.Russian
				case task.VocabSubtypeFillBlank:
					// Prompt already carries the blanked sentence.
				default:
					v.Word = word.English
					v.Transcript = word.Transcription
					v.Example = word.Example
					v.AudioText, v.AudioURL = word.English, word.AudioURL
				}
			}
		}
	}
	return v
}

func (s *Server) handleTaskAnswer(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad task id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	t, err := s.taskSvc.GetTaskByID(r.Context(), taskID)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	var result *task.TaskResult
	switch {
	case r.FormValue("skip") != "":
		result, err = s.taskSvc.SubmitSkip(r.Context(), taskID, u.ID)
	case isTyping(t):
		result, err = s.taskSvc.SubmitTypedAnswer(r.Context(), taskID, u.ID, r.FormValue("answer"))
	default:
		result, err = s.taskSvc.SubmitAnswer(r.Context(), taskID, u.ID, r.FormValue("answer"))
	}

	// Answering something already answered is not an error the user should see:
	// it happens naturally when the same round is open in the bot and the
	// browser. Show the current state instead of a failure.
	if errors.Is(err, task.ErrAlreadyAnswered) {
		slog.Info("web: task already answered elsewhere", "user_id", u.ID, "task_id", taskID)
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		slog.Error("web: submit answer", "user_id", u.ID, "task_id", taskID, "error", err)
		http.Error(w, "submit failed", http.StatusInternalServerError)
		return
	}

	if result.Score > 0 {
		if err := s.scoringSvc.RecordScore(r.Context(), u.ID, t.GroupID, t.Level, result.Score); err != nil {
			slog.Error("web: record score", "user_id", u.ID, "error", err)
		}
	}

	fb := feedbackView{
		Correct: result.Correct,
		Skipped: result.Skipped,
		Answer:  t.Answer,
		Score:   result.Score,
		Level:   t.Level,
	}
	if t.Type == task.TypeGrammar && t.Metadata != nil && showRule(u, result.Correct) {
		// The generator already stamps the category explanation onto the task,
		// so no repository lookup is needed here.
		fb.Explanation, _ = t.Metadata["explanation"].(string)
	}

	// Look ahead so the feedback can carry either a "next" button or the round
	// summary, saving the user a round trip.
	ds, err := s.taskSvc.GetTodaySet(r.Context(), u.ID, t.GroupID)
	if err == nil && ds != nil {
		if next, err := s.taskSvc.GetNextUnanswered(r.Context(), ds.ID, u.ID); err == nil {
			if next != nil {
				fb.HasNext = true
			} else {
				total, correct := 0, 0
				if _, tt, err := s.taskSvc.GetProgress(r.Context(), ds.ID, u.ID); err == nil {
					total = tt
				}
				if c, err := s.taskSvc.GetCorrectCount(r.Context(), ds.ID, u.ID); err == nil {
					correct = c
				}
				fb.Done = &roundSummary{Correct: correct, Total: total}
			}
		}
	}

	s.render(w, "task_feedback", fb)
}

// handleTaskNext renders the next slot as a fragment, so answering a task never
// reloads the page.
//
// It reads the *current* set rather than generating one: GenerateDaily opens
// the next round once the current one is fully answered, so calling it here
// would silently roll the user into a new round instead of showing the summary.
// Starting another round is an explicit act — the "Ещё раунд" link.
func (s *Server) handleTaskNext(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	ds, err := s.currentSet(r, u)
	if err != nil {
		slog.Error("web: next slot set", "user_id", u.ID, "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	if ds == nil {
		s.render(w, "task_done", &roundSummary{})
		return
	}

	slot, summary, err := s.nextSlot(r, u, ds)
	if err != nil {
		slog.Error("web: next slot", "user_id", u.ID, "error", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	if slot == nil {
		s.render(w, "task_done", summary)
		return
	}
	s.render(w, "task_slot", slot)
}

func isTyping(t *task.Task) bool {
	if t.Type != task.TypeVocab || t.Metadata == nil {
		return false
	}
	subtype, _ := t.Metadata["subtype"].(string)
	return task.IsVocabTypingSubtype(subtype)
}

// taskInstruction is the line above the prompt. Unlike the bot's version it
// carries no emoji — in the web the mode is conveyed by the card itself.
func taskInstruction(t *task.Task) string {
	if t.Type == task.TypeGrammar {
		return "Выберите вариант, чтобы заполнить пропуск"
	}
	if t.Metadata == nil {
		return "Выберите перевод"
	}
	switch t.Metadata["subtype"] {
	case task.VocabSubtypeTypingEnRu:
		return "Введите перевод на русский"
	case task.VocabSubtypeTypingRuEn:
		return "Введите перевод на английский"
	case task.VocabSubtypeFillBlank:
		return "Выберите слово, подходящее по смыслу"
	default:
		return "Выберите перевод"
	}
}

func metaInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		id, _ := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return id
	default:
		return 0
	}
}

// showRule decides whether answer feedback carries the grammar rule. After a
// mistake it always does — that is the moment it helps. After a correct answer
// only when the user asked for it in the profile settings.
func showRule(u *user.User, correct bool) bool {
	return !correct || (u != nil && u.ShowRuleOnCorrect)
}
