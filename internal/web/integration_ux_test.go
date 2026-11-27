package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
	"makeMe/internal/seed"
	"makeMe/internal/storage/sqlc"
)

// seedRules loads the shipped rule texts the way app startup does. Migrations
// leave grammar_categories.explanation empty, so without this every rule
// check would pass vacuously.
func seedRules(t *testing.T, env *dbEnv) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	file := filepath.Join(filepath.Dir(thisFile), "..", "..", "data", "grammar_categories.json")
	require.NoError(t, seed.GrammarCategories(context.Background(), sqlc.New(env.pool), file))
}

// postPlainForm submits a form the way a browser does without htmx.
func (e *dbEnv) postPlainForm(t *testing.T, path, form string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c != nil {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func (e *dbEnv) setShowRule(t *testing.T, c *http.Cookie, on bool) {
	t.Helper()
	v := "0"
	if on {
		v = "1"
	}
	rr := e.postPlainForm(t, "/profile/settings", url.Values{"show_rule_on_correct": {v}}.Encode(), c)
	require.Equal(t, http.StatusSeeOther, rr.Code, rr.Body.String())
}

// --- settings ------------------------------------------------------------

func TestProfileSettingShowRuleOnCorrectIsSaved(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 700, user.A1)
	ctx := context.Background()

	require.False(t, u.ShowRuleOnCorrect, "the setting is off by default")
	body := env.get(t, "/profile", cookie).Body.String()
	require.Contains(t, body, "Показывать правило и после верного ответа")
	require.Contains(t, body, `aria-checked="false"`)
	require.Contains(t, body, `name="show_rule_on_correct" value="1"`,
		"pressing the switch while off must turn it on")

	rr := env.postPlainForm(t, "/profile/settings", "show_rule_on_correct=1", cookie)
	require.Equal(t, http.StatusSeeOther, rr.Code)
	require.Equal(t, "/profile?saved=1", rr.Header().Get("Location"))

	got, err := env.userSvc.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.True(t, got.ShowRuleOnCorrect, "the setting must persist")

	body = env.get(t, "/profile?saved=1", cookie).Body.String()
	require.Contains(t, body, `aria-checked="true"`)
	require.Contains(t, body, "Настройка сохранена")
	require.Contains(t, body, `name="show_rule_on_correct" value="0"`)

	env.setShowRule(t, cookie, false)
	got, err = env.userSvc.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.False(t, got.ShowRuleOnCorrect)

	// Only an explicit value is accepted.
	rr = env.postPlainForm(t, "/profile/settings", "show_rule_on_correct=yes", cookie)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	rr = env.postPlainForm(t, "/profile/settings", "", cookie)
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// Same protection as every other private route.
	rr = env.postPlainForm(t, "/profile/settings", "show_rule_on_correct=1", nil)
	require.Equal(t, http.StatusSeeOther, rr.Code)
	require.Equal(t, "/login", rr.Header().Get("Location"))
}

// --- tasks: rule after a correct answer, prompt escaping ------------------

// nextGrammarTask walks the round, skipping vocab tasks and grammar tasks
// without a rule, until it reaches a grammar task whose rule is not empty.
// It returns the rendered slot and the task.
func nextGrammarTask(t *testing.T, env *dbEnv, cookie *http.Cookie, userID int64) (string, *task.Task) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		body := env.get(t, "/tasks", cookie).Body.String()
		require.Contains(t, body, `hx-post="/tasks/`, "round ran out before a grammar task with a rule")
		tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, currentTaskID(t, body)))
		require.NoError(t, err)
		if expl, _ := tk.Metadata["explanation"].(string); tk.Type == task.TypeGrammar && expl != "" {
			return body, tk
		}
		_, err = env.taskSvc.SubmitSkip(ctx, tk.ID, userID)
		require.NoError(t, err)
	}
	t.Fatal("no grammar task with a rule in the round")
	return "", nil
}

func TestTaskFeedbackRuleFollowsSetting(t *testing.T) {
	env := setupDBEnv(t)
	seedRules(t, env)
	u, cookie := env.signedIn(t, 701, user.A1)
	ctx := context.Background()

	// Every grammar prompt gets an apostrophe, so whichever grammar task comes up
	// shows whether the web escapes it once or twice.
	_, err := env.pool.Exec(ctx, `UPDATE grammar_questions SET prompt = prompt || ' (it''s)'`)
	require.NoError(t, err)

	answer := func(tk *task.Task, value string) string {
		rr := env.postForm(t, "/tasks/"+strconv.FormatInt(tk.ID, 10)+"/answer", answerForm(value), cookie)
		require.Equal(t, http.StatusOK, rr.Code)
		return rr.Body.String()
	}

	// Off (default), correct: no rule.
	slot, tk := nextGrammarTask(t, env, cookie, u.ID)
	require.Contains(t, slot, "(it&#39;s)", "the prompt must reach the page escaped exactly once")
	require.NotContains(t, slot, "&amp;#39;", "the prompt must not be escaped twice")
	fb := answer(tk, tk.Answer)
	require.Contains(t, fb, "Верно")
	require.NotContains(t, fb, "ruledoc", "with the setting off a correct answer shows no rule")

	// Wrong: the rule is always there.
	_, tk = nextGrammarTask(t, env, cookie, u.ID)
	fb = answer(tk, "заведомо неверно")
	require.Contains(t, fb, "Неверно")
	require.Contains(t, fb, `<div class="ruledoc">`, "a wrong answer always shows the rule")

	// On, correct: the rule is shown.
	env.setShowRule(t, cookie, true)
	_, tk = nextGrammarTask(t, env, cookie, u.ID)
	fb = answer(tk, tk.Answer)
	require.Contains(t, fb, "Верно")
	require.Contains(t, fb, `<div class="ruledoc">`, "with the setting on a correct answer shows the rule")
}

// --- learn: rule after a correct answer -----------------------------------

func TestLearnFeedbackRuleFollowsSetting(t *testing.T) {
	env := setupDBEnv(t)
	seedRules(t, env)
	_, cookie := env.signedIn(t, 702, user.A1)
	ctx := context.Background()

	topic := firstTopic(t, env.get(t, "/learn", cookie).Body.String())
	questions, err := env.grammarRepo.GetQuestionsByCategory(ctx, topic)
	require.NoError(t, err)
	sample := task.StratifiedSample(questions, 777, 20)
	require.NotEmpty(t, sample)
	require.NotEmpty(t, sample[0].Explanation, "the first A1 topic must carry a rule for this test")

	form := url.Values{"name": {topic}, "seed": {"777"}, "i": {"0"}, "run": {"0"}, "answer": {sample[0].Answer}}

	off := env.postForm(t, "/learn/answer", form.Encode(), cookie).Body.String()
	require.Contains(t, off, "Верно")
	require.NotContains(t, off, "ruledoc")

	env.setShowRule(t, cookie, true)
	on := env.postForm(t, "/learn/answer", form.Encode(), cookie).Body.String()
	require.Contains(t, on, "Верно")
	require.Contains(t, on, `<div class="ruledoc">`)

	form.Set("answer", "заведомо неверно")
	wrong := env.postForm(t, "/learn/answer", form.Encode(), cookie).Body.String()
	require.Contains(t, wrong, `<div class="ruledoc">`)
}

// The topic page draws the rule in blocks too.
func TestLearnTopicRendersRuleBlocks(t *testing.T) {
	env := setupDBEnv(t)
	seedRules(t, env)
	_, cookie := env.signedIn(t, 703, user.A1)

	topic := firstTopic(t, env.get(t, "/learn", cookie).Body.String())
	body := env.get(t, "/learn/topic?name="+url.QueryEscape(topic), cookie).Body.String()
	require.Contains(t, body, `<div class="ruledoc">`)
	require.Contains(t, body, `class="ruledoc-lead"`)
	require.Contains(t, body, `class="ruledoc-sec"`)
}

// --- words: listen button and auto-advance --------------------------------

// The listen button must not appear where the English word is the answer, and
// must appear — with the word to speak — where the English word is shown.
func TestWordsListenButtonHiddenWhereItGivesTheAnswer(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 704, user.A1)
	ctx := context.Background()

	seen := map[words.Mode]bool{}
	body := env.get(t, "/words", cookie).Body.String()
	for i := 0; i < 80 && len(seen) < 4; i++ {
		if !strings.Contains(body, `hx-post="/words/`) {
			break
		}
		id, mode := currentWordSlot(t, body)
		word, err := env.srsRepo.GetWordByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)

		switch words.Mode(mode) {
		case words.ModeRuEnMC, words.ModeRuEnTyping:
			require.NotContains(t, body, `data-action="play-audio"`,
				"%s must not offer to play the answer", mode)
		default:
			require.Contains(t, body, `data-text="`+word.English+`"`,
				"%s must offer to hear the shown word", mode)
		}
		seen[words.Mode(mode)] = true

		fb := env.postForm(t, "/words/"+id+"/answer",
			url.Values{"mode": {mode}, "answer": {words.ExpectedAnswer(word, words.Mode(mode))}}.Encode(), cookie)
		require.Equal(t, http.StatusOK, fb.Code)
		// After the answer the word may be heard in every mode.
		require.Contains(t, fb.Body.String(), `data-text="`+word.English+`"`)

		body = env.get(t, "/words/next", cookie).Body.String()
	}
	require.True(t, seen[words.ModeRuEnMC] || seen[words.ModeRuEnTyping], "no RU→EN drill came up: %v", seen)
	require.True(t, seen[words.ModeEnRuMC] || seen[words.ModeEnRuTyping], "no EN→RU drill came up: %v", seen)
}

// Correct answers move on after a short beat; a learned word waits for the
// button.
func TestWordsFeedbackAutoAdvanceUnlessLearned(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 705, user.A1)
	ctx := context.Background()

	sawLearned := false
	body := env.get(t, "/words", cookie).Body.String()
	for i := 0; i < 90 && !sawLearned; i++ {
		require.Contains(t, body, `hx-post="/words/`, "session ended before a word was learned")
		id, mode := currentWordSlot(t, body)
		word, err := env.srsRepo.GetWordByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)

		fb := env.postForm(t, "/words/"+id+"/answer",
			url.Values{"mode": {mode}, "answer": {words.ExpectedAnswer(word, words.Mode(mode))}}.Encode(), cookie).Body.String()
		require.Contains(t, fb, "Верно")
		require.Contains(t, fb, "data-advance-next", "«Дальше» must be marked for Enter and the timer")
		if strings.Contains(fb, "Слово выучено") {
			sawLearned = true
			require.NotContains(t, fb, "data-autoadvance", "a learned word waits for the button")
		} else {
			require.Contains(t, fb, `data-autoadvance="1000"`)
		}
		body = env.get(t, "/words/next", cookie).Body.String()
	}
	require.True(t, sawLearned, "no word got learned")
}

// A wrong answer moves on after long enough to read the right one — until the
// card offers to defer the word, which waits for a decision.
func TestWordsFeedbackAutoAdvanceUnlessDeferOffered(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 706, user.A1)

	body := env.get(t, "/words", cookie).Body.String()
	id, mode := currentWordSlot(t, body)

	for i := 0; i <= words.DeferThreshold; i++ {
		fb := env.postForm(t, "/words/"+id+"/answer",
			url.Values{"mode": {mode}, "answer": {"заведомо неверно"}}.Encode(), cookie).Body.String()
		if strings.Contains(fb, "Отложить до завтра") {
			require.Equal(t, words.DeferThreshold, i, "defer offered too early")
			require.NotContains(t, fb, "data-autoadvance", "the defer offer waits for a decision")
		} else {
			require.Contains(t, fb, `data-autoadvance="3000"`)
		}
	}
}

// --- tasks: listen button -------------------------------------------------

func TestTaskListenButtonHiddenWhereItGivesTheAnswer(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 707, user.A1)
	ctx := context.Background()

	hidden, shown := 0, 0
	for i := 0; i < 60 && (hidden == 0 || shown == 0); i++ {
		body := env.get(t, "/tasks", cookie).Body.String()
		if !strings.Contains(body, `hx-post="/tasks/`) {
			break
		}
		tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, currentTaskID(t, body)))
		require.NoError(t, err)

		if tk.Type == task.TypeVocab {
			switch tk.Metadata["subtype"] {
			case task.VocabSubtypeRuEn, task.VocabSubtypeTypingRuEn, task.VocabSubtypeFillBlank:
				require.NotContains(t, body, `data-action="play-audio"`,
					"%v must not offer to play the answer", tk.Metadata["subtype"])
				hidden++
			default:
				require.Contains(t, body, `data-action="play-audio"`, "%v", tk.Metadata["subtype"])
				shown++
			}
		} else {
			require.NotContains(t, body, `data-action="play-audio"`, "grammar tasks have no word to hear")
		}
		_, err = env.taskSvc.SubmitSkip(ctx, tk.ID, u.ID)
		require.NoError(t, err)
	}
	require.NotZero(t, hidden, "no RU→EN or fill-blank task came up")
	require.NotZero(t, shown, "no EN→RU task came up")
}
