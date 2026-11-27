package web

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
)

// nextSlotTemplate returns what the feedback carries inside
// <template data-next-slot>, and whether it carries one at all.
func nextSlotTemplate(body string) (string, bool) {
	const open = "<template data-next-slot>"
	i := strings.Index(body, open)
	if i < 0 {
		return "", false
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, "</template>")
	if j < 0 {
		return "", false
	}
	return rest[:j], true
}

// The feedback brings the next task along, so "Дальше" needs no second
// request. What it brings must be exactly the slot /tasks/next would render —
// options only, no answer, no rule.
func TestTaskFeedbackCarriesNextSlot(t *testing.T) {
	env := setupDBEnv(t)
	seedRules(t, env)
	_, cookie := env.signedIn(t, 800, user.A1)
	ctx := context.Background()

	body := env.get(t, "/tasks", cookie).Body.String()
	for answered := 0; answered < 10; answered++ {
		id := currentTaskID(t, body)
		tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)

		// Wrong on purpose: the feedback then carries this task's rule, and
		// the template must still carry nothing of the next one's.
		fb := env.postForm(t, "/tasks/"+id+"/answer", answerForm("заведомо неверно"), cookie).Body.String()
		tpl, has := nextSlotTemplate(fb)

		next := env.get(t, "/tasks/next", cookie).Body.String()
		if answered == 9 {
			require.False(t, has, "the last task of the round has nothing to prefetch")
			require.Contains(t, fb, "верных ответов", "the round summary stays in the feedback")
			require.Contains(t, next, "верных ответов")
			return
		}
		require.True(t, has, "feedback for task %d must carry the next slot", answered+1)
		require.Equal(t, strings.TrimSpace(next), strings.TrimSpace(tpl),
			"the prefetched slot must be the one /tasks/next renders")

		nextTask, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, currentTaskID(t, next)))
		require.NoError(t, err)
		require.NotEqual(t, tk.ID, nextTask.ID)
		require.Contains(t, tpl, `hx-post="/tasks/`+strconv.FormatInt(nextTask.ID, 10)+`/answer"`)
		require.NotContains(t, tpl, "ruledoc", "the next task's rule must not travel with the slot")
		if isTyping(nextTask) {
			require.NotContains(t, tpl, nextTask.Answer, "a typing task's answer must not reach the browser")
		}
		body = next
	}
}

// A prefetched task answered meanwhile in the bot: submitting it from the
// browser refreshes the page instead of failing or scoring twice.
func TestPrefetchedTaskAnsweredElsewhereRefreshes(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 801, user.A1)
	ctx := context.Background()

	body := env.get(t, "/tasks", cookie).Body.String()
	id := currentTaskID(t, body)
	fb := env.postForm(t, "/tasks/"+id+"/answer", "skip=1", cookie).Body.String()
	tpl, has := nextSlotTemplate(fb)
	require.True(t, has)

	nextID := currentTaskID(t, tpl)
	nextTask, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, nextID))
	require.NoError(t, err)
	_, err = env.taskSvc.SubmitAnswer(ctx, nextTask.ID, u.ID, nextTask.Answer) // the bot
	require.NoError(t, err)

	rr := env.postForm(t, "/tasks/"+nextID+"/answer", answerForm(nextTask.Answer), cookie)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, "true", rr.Header().Get("HX-Refresh"))
}

// /words: the next drill travels with the feedback — the one /words/next
// would show — except when the card offers to defer the word.
func TestWordFeedbackCarriesNextSlotUnlessDeferOffered(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 802, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	id, mode := currentWordSlot(t, body)
	word, err := env.srsRepo.GetWordByID(ctx, taskIDFrom(t, id))
	require.NoError(t, err)

	fb := env.postForm(t, "/words/"+id+"/answer",
		url.Values{"mode": {mode}, "answer": {words.ExpectedAnswer(word, words.Mode(mode))}}.Encode(), cookie).Body.String()
	tpl, has := nextSlotTemplate(fb)
	require.True(t, has, "a plain answer must carry the next drill")

	next := env.get(t, "/words/next", cookie).Body.String()
	tplID, tplMode := currentWordSlot(t, tpl)
	nextID, nextMode := currentWordSlot(t, next)
	require.Equal(t, nextID, tplID, "the prefetched drill must be the word /words/next picks")
	require.Equal(t, nextMode, tplMode, "…in the same mode")

	// Past the defer threshold the next drill depends on the user's choice.
	id, mode = nextID, nextMode
	var last string
	for i := 0; i <= words.DeferThreshold; i++ {
		last = env.postForm(t, "/words/"+id+"/answer",
			url.Values{"mode": {mode}, "answer": {"заведомо неверно"}}.Encode(), cookie).Body.String()
	}
	require.Contains(t, last, "Отложить до завтра")
	_, has = nextSlotTemplate(last)
	require.False(t, has, "with the defer offer on screen nothing is prefetched")
}

// When the answer finishes the session there is nothing to prefetch, and the
// prefetch must not close the session behind the user's back: closing still
// happens on their own "Дальше".
func TestWordFeedbackOnLastDrillLeavesSessionOpen(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 803, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	firstID, _ := currentWordSlot(t, body)

	session, err := env.wordsSvc.RepoOpenSession(ctx, u.ID)
	require.NoError(t, err)
	for _, wid := range session.WordIDs {
		if strconv.FormatInt(wid, 10) != firstID {
			require.NoError(t, env.wordsSvc.MarkAlreadyKnown(ctx, session, wid))
		}
	}

	// One word left: drill it through every mode.
	body = env.get(t, "/words/next", cookie).Body.String()
	for i := 0; i < 4*words.ThresholdPerMode; i++ {
		id, mode := currentWordSlot(t, body)
		require.Equal(t, firstID, id)
		word, err := env.srsRepo.GetWordByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)

		fb := env.postForm(t, "/words/"+id+"/answer",
			url.Values{"mode": {mode}, "answer": {words.ExpectedAnswer(word, words.Mode(mode))}}.Encode(), cookie).Body.String()
		tpl, has := nextSlotTemplate(fb)

		if i < 4*words.ThresholdPerMode-1 {
			require.True(t, has, "drill %d: more modes remain, the next one must be prefetched", i+1)
			body = tpl
			continue
		}

		require.Contains(t, fb, "Слово выучено")
		require.False(t, has, "nothing is left to prefetch")
		open, err := env.wordsSvc.RepoOpenSession(ctx, u.ID)
		require.NoError(t, err)
		require.NotNil(t, open, "rendering the feedback must not close the session")
		require.Equal(t, session.ID, open.ID)
	}

	done := env.get(t, "/words/next", cookie).Body.String()
	require.Contains(t, done, "Сессия закрыта")
	open, err := env.wordsSvc.RepoOpenSession(ctx, u.ID)
	require.NoError(t, err)
	require.Nil(t, open, "the user's own «Дальше» closes the exhausted session")
}
