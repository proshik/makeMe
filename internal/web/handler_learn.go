package web

import (
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
)

// learnPracticeLimit matches the bot: one practice run is at most this many
// questions, sampled across subtypes so a single sitting covers different
// aspects of the rule rather than twenty affirmatives in a row.
const learnPracticeLimit = 20

type learnTopic struct {
	Name      string
	Level     user.Level
	Questions int
}

type learnIndexPage struct {
	User   *user.User
	Groups []learnLevelGroup
}

type learnLevelGroup struct {
	Level  user.Level
	Topics []learnTopic
}

type learnTopicPage struct {
	User        *user.User
	Topic       string
	Explanation string
	Total       int
	Seed        int64
}

// learnSlot is one question of a practice run. The run carries no server-side
// state: the seed plus the index reproduce the same sample every time, which
// is why the sequence survives a refresh.
type learnSlot struct {
	Topic      string
	Seed       int64
	Index      int // 0-based
	Number     int // 1-based, for display
	Total      int
	Question   task.GrammarQuestion
	CorrectRun int // running tally of correct answers in this run
}

type learnFeedback struct {
	Topic       string
	Seed        int64
	NextIndex   int
	Total       int
	Correct     bool
	Chosen      string
	Answer      string
	Explanation string
	Done        bool
	CorrectRun  int
}

func (s *Server) handleLearn(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	// The user's level and the one above it: enough to practise what they are
	// on and to look ahead, without listing the whole ladder.
	levels := []user.Level{u.Level}
	if next, ok := nextLevel(u.Level); ok {
		levels = append(levels, next)
	}

	page := learnIndexPage{User: u}
	for _, level := range levels {
		cats, err := s.grammarRepo.GetCategoriesByLevel(r.Context(), level)
		if err != nil {
			slog.Error("web: list grammar categories", "level", level, "error", err)
			continue
		}
		group := learnLevelGroup{Level: level}
		for _, c := range cats {
			questions, err := s.grammarRepo.GetQuestionsByCategory(r.Context(), c.Name)
			if err != nil {
				continue
			}
			group.Topics = append(group.Topics, learnTopic{
				Name: c.Name, Level: level, Questions: len(questions),
			})
		}
		if len(group.Topics) > 0 {
			page.Groups = append(page.Groups, group)
		}
	}

	s.render(w, "learn.html", page)
}

// handleLearnTopic shows the theory and how many questions the run will have.
// A fresh seed is drawn on every entry, so re-opening a topic reshuffles the
// order — the same behaviour the bot has.
func (s *Server) handleLearnTopic(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	topic := topicOf(r)
	if topic == "" {
		s.handleNotFound(w, r)
		return
	}

	explanation, questions, err := s.loadTopic(r, topic)
	if err != nil {
		slog.Error("web: load grammar topic", "topic", topic, "error", err)
		s.renderError(w, "Не удалось загрузить тему. Попробуйте позже.")
		return
	}
	if len(questions) == 0 {
		s.handleNotFound(w, r)
		return
	}

	// Один seed и на подсчёт вопросов, и на сам прогон: свежий при каждом
	// входе в тему, поэтому порядок перемешивается, как и в боте.
	seed := int64(rand.Uint32())
	s.render(w, "learn_topic.html", learnTopicPage{
		User:        u,
		Topic:       topic,
		Explanation: explanation,
		Total:       len(sampleFor(questions, seed)),
		Seed:        seed,
	})
}

func (s *Server) handleLearnPractice(w http.ResponseWriter, r *http.Request) {
	topic := topicOf(r)
	seed, _ := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
	index, _ := strconv.Atoi(r.URL.Query().Get("i"))

	slot, ok, err := s.learnSlotAt(r, topic, seed, index)
	if err != nil {
		slog.Error("web: learn practice", "topic", topic, "error", err)
		s.renderError(w, "Не удалось загрузить вопрос. Попробуйте позже.")
		return
	}
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	s.render(w, "learn_practice.html", slot)
}

func (s *Server) handleLearnAnswer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	topic := topicOf(r)
	seed, _ := strconv.ParseInt(r.FormValue("seed"), 10, 64)
	index, _ := strconv.Atoi(r.FormValue("i"))
	chosen := r.FormValue("answer")
	correctRun, _ := strconv.Atoi(r.FormValue("run"))

	slot, ok, err := s.learnSlotAt(r, topic, seed, index)
	if err != nil || !ok {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	correct := chosen == slot.Question.Answer
	if correct {
		correctRun++
	}

	// /learn is practice, not assessment: nothing is scored and nothing is
	// written to the SRS. It exists so a rule can be drilled on demand.
	fb := learnFeedback{
		Topic:       topic,
		Seed:        seed,
		NextIndex:   index + 1,
		Total:       slot.Total,
		Correct:     correct,
		Chosen:      chosen,
		Answer:      slot.Question.Answer,
		Explanation: slot.Question.Explanation,
		Done:        index+1 >= slot.Total,
		CorrectRun:  correctRun,
	}
	s.render(w, "learn_feedback", fb)
}

// handleLearnNext renders the next question of a run as a fragment.
func (s *Server) handleLearnNext(w http.ResponseWriter, r *http.Request) {
	topic := topicOf(r)
	seed, _ := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
	index, _ := strconv.Atoi(r.URL.Query().Get("i"))
	run, _ := strconv.Atoi(r.URL.Query().Get("run"))

	slot, ok, err := s.learnSlotAt(r, topic, seed, index)
	if err != nil || !ok {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	slot.CorrectRun = run
	s.render(w, "learn_slot", slot)
}

func (s *Server) loadTopic(r *http.Request, topic string) (explanation string, questions []task.GrammarQuestion, err error) {
	questions, err = s.grammarRepo.GetQuestionsByCategory(r.Context(), topic)
	if err != nil {
		return "", nil, err
	}
	// The explanation lives on the category; find it via the levels present in
	// the questions rather than assuming the user's own level, since a topic
	// may be listed one level up.
	seen := map[user.Level]bool{}
	for _, q := range questions {
		if seen[q.Level] {
			continue
		}
		seen[q.Level] = true
		cats, err := s.grammarRepo.GetCategoriesByLevel(r.Context(), q.Level)
		if err != nil {
			continue
		}
		for _, c := range cats {
			if c.Name == topic {
				return c.Explanation, questions, nil
			}
		}
	}
	return "", questions, nil
}

func (s *Server) learnSlotAt(r *http.Request, topic string, seed int64, index int) (*learnSlot, bool, error) {
	if topic == "" || index < 0 {
		return nil, false, nil
	}
	all, err := s.grammarRepo.GetQuestionsByCategory(r.Context(), topic)
	if err != nil {
		return nil, false, err
	}
	sample := sampleFor(all, seed)
	if index >= len(sample) {
		return nil, false, nil
	}
	return &learnSlot{
		Topic:    topic,
		Seed:     seed,
		Index:    index,
		Number:   index + 1,
		Total:    len(sample),
		Question: sample[index],
	}, true, nil
}

func sampleFor(all []task.GrammarQuestion, seed int64) []task.GrammarQuestion {
	return task.StratifiedSample(all, seed, learnPracticeLimit)
}

// topicOf reads the category from the query. Category names carry spaces and
// slashes ("can/can\'t"), which a path segment would mangle — a query value
// carries them intact.
func topicOf(r *http.Request) string {
	if r.Method == http.MethodPost {
		return r.FormValue("name")
	}
	return r.URL.Query().Get("name")
}

func nextLevel(l user.Level) (user.Level, bool) {
	levels := user.AllLevels()
	if i := l.Index(); i >= 0 && i+1 < len(levels) {
		return levels[i+1], true
	}
	return "", false
}
