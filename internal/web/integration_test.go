package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
)

// dbEnv is a full stack over a real Postgres: the same services the bot uses,
// plus the web router on top. No WireMock here — the web adapter never calls
// the Telegram API.
type dbEnv struct {
	router      *chi.Mux
	jwt         *JWTService
	userSvc     *user.Service
	taskSvc     *task.Service
	assessSvc   *assessment.Service
	wordsSvc    *words.Service
	srsRepo     srs.Repository
	grammarRepo task.GrammarRepository
	pool        *pgxpool.Pool
}

func setupDBEnv(t *testing.T) *dbEnv {
	t.Helper()
	ctx := context.Background()

	pg, err := tcpostgres.Run(ctx, "postgres:17",
		tcpostgres.WithDatabase("makeme_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dbURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	_, thisFile, _, _ := runtime.Caller(0)
	require.NoError(t, goose.Up(db, filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")))
	require.NoError(t, db.Close())

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	q := sqlc.New(pool)
	userSvc := user.NewService(postgres.NewUserRepository(q))
	groupSvc := group.NewService(postgres.NewGroupRepository(q))
	srsRepo := postgres.NewSRSRepository(q)
	grammarRepo := postgres.NewGrammarRepository(q)
	taskSvc := task.NewService(
		postgres.NewTaskRepository(q), srsRepo, 5,
		task.NewVocabGenerator(srsRepo), task.NewGrammarGenerator(grammarRepo),
	)
	scoringSvc := scoring.NewService(postgres.NewScoringRepository(q))
	assessSvc := assessment.NewService(postgres.NewAssessmentRepository(q), userSvc, srsRepo, grammarRepo)
	wordsSvc := words.NewService(postgres.NewWordsRepository(q), srsRepo)

	jwtSvc, err := NewJWTService(testSecret(t), 24)
	require.NoError(t, err)

	// The OIDC endpoint is never reached in these tests — the session cookie is
	// minted directly.
	srv, err := New(Deps{
		UserSvc: userSvc, GroupSvc: groupSvc, TaskSvc: taskSvc,
		ScoringSvc: scoringSvc, SRSRepo: srsRepo, AssessSvc: assessSvc, WordsSvc: wordsSvc, GrammarRepo: grammarRepo,
	}, jwtSvc, NewOIDCClient("http://unused", "client-1", "secret", "http://app/cb"), false)
	require.NoError(t, err)

	r := chi.NewRouter()
	srv.Mount(r)

	return &dbEnv{router: r, jwt: jwtSvc, userSvc: userSvc, taskSvc: taskSvc,
		assessSvc: assessSvc, wordsSvc: wordsSvc, srsRepo: srsRepo,
		grammarRepo: grammarRepo, pool: pool}
}

// signedIn creates an onboarded user and returns them with a session cookie.
func (e *dbEnv) signedIn(t *testing.T, telegramID int64, level user.Level) (*user.User, *http.Cookie) {
	t.Helper()
	ctx := context.Background()

	u, err := e.userSvc.Register(ctx, telegramID, "alice", "Алиса")
	require.NoError(t, err)
	require.NoError(t, e.userSvc.UpdateLevel(ctx, u.ID, level))

	u, err = e.userSvc.GetByID(ctx, u.ID)
	require.NoError(t, err)

	token, err := e.jwt.Create(u.ID)
	require.NoError(t, err)
	return u, &http.Cookie{Name: authCookie, Value: token}
}

func (e *dbEnv) get(t *testing.T, path string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(c)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func (e *dbEnv) postForm(t *testing.T, path string, form string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(c)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

func TestTasksRequiresLevelBeforeStarting(t *testing.T) {
	env := setupDBEnv(t)
	ctx := context.Background()

	u, err := env.userSvc.Register(ctx, 100, "bob", "Борис")
	require.NoError(t, err)
	token, err := env.jwt.Create(u.ID)
	require.NoError(t, err)

	rr := env.get(t, "/tasks", &http.Cookie{Name: authCookie, Value: token})
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "тест на уровень",
		"a user without a level must be sent to the assessment, not to tasks")
}

func TestTasksAutoCreatesSoloGroup(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 101, user.A1)

	rr := env.get(t, "/tasks", cookie)
	require.Equal(t, http.StatusOK, rr.Code)

	// Without the solo group there would be nowhere for the score to land, and
	// the first web-only user would hit a dead end.
	var groups int
	require.NoError(t, env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM memberships WHERE user_id = $1`, u.ID).Scan(&groups))
	require.Equal(t, 1, groups)
}

// currentTaskID digs the task id out of the rendered slot.
func currentTaskID(t *testing.T, body string) string {
	t.Helper()
	const marker = `hx-post="/tasks/`
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "no task form in the rendered slot")
	rest := body[i+len(marker):]
	return rest[:strings.Index(rest, "/answer")]
}

func TestTasksFullRoundScoresAndFinishes(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 102, user.A1)
	ctx := context.Background()

	rr := env.get(t, "/tasks", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	answered := 0
	for answered < 20 { // safety bound; a round is 10
		if !strings.Contains(body, `hx-post="/tasks/`) {
			break
		}
		id := currentTaskID(t, body)

		// Answer correctly by reading the answer straight from the domain —
		// this test is about the web flow, not about knowing English.
		tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)

		fb := env.postForm(t, "/tasks/"+id+"/answer", answerForm(tk.Answer), cookie)
		require.Equal(t, http.StatusOK, fb.Code)
		require.Contains(t, fb.Body.String(), "Верно",
			"a correct answer must be reported as correct")
		answered++

		next := env.get(t, "/tasks/next", cookie)
		require.Equal(t, http.StatusOK, next.Code)
		body = next.Body.String()
	}

	require.Equal(t, 10, answered, "a round is ten tasks")
	require.Contains(t, body, "верных ответов", "the round should end with a summary")

	// Every correct answer must have produced a score row.
	var raw int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT coalesce(sum(raw),0) FROM scores WHERE user_id = $1`, u.ID).Scan(&raw))
	require.Greater(t, raw, 0, "correct answers must be scored")
}

func TestTasksWrongAnswerShowsCorrectOne(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 103, user.A1)

	body := env.get(t, "/tasks", cookie).Body.String()
	id := currentTaskID(t, body)

	fb := env.postForm(t, "/tasks/"+id+"/answer", answerForm("заведомо неверный ответ"), cookie)
	require.Equal(t, http.StatusOK, fb.Code)
	require.Contains(t, fb.Body.String(), "Неверно")
	require.Contains(t, fb.Body.String(), "Правильный ответ")
}

func TestTasksSkipIsRecordedSeparately(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 104, user.A1)

	body := env.get(t, "/tasks", cookie).Body.String()
	id := currentTaskID(t, body)

	fb := env.postForm(t, "/tasks/"+id+"/answer", "skip=1", cookie)
	require.Equal(t, http.StatusOK, fb.Code)
	require.Contains(t, fb.Body.String(), "Пропущено")

	var skipped bool
	require.NoError(t, env.pool.QueryRow(context.Background(),
		`SELECT skipped FROM task_results WHERE user_id = $1`, u.ID).Scan(&skipped))
	require.True(t, skipped, "an honest skip must be distinguishable from a wrong guess")
}

// The headline claim of the whole feature: a round begun in the bot continues
// in the browser at the right place, because both read the same rows.
func TestRoundStartedInBotResumesInBrowser(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 105, user.A1)
	ctx := context.Background()

	// "The bot": generate the round and answer three tasks through the domain
	// service directly, with no web request involved.
	groupSvc := group.NewService(postgres.NewGroupRepository(sqlc.New(env.pool)))
	g, err := groupSvc.Create(ctx, "Соло", u.ID)
	require.NoError(t, err)

	ds, err := env.taskSvc.GenerateDaily(ctx, u.ID, g.ID, user.A1)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		next, err := env.taskSvc.GetNextUnanswered(ctx, ds.ID, u.ID)
		require.NoError(t, err)
		require.NotNil(t, next)
		_, err = env.taskSvc.SubmitAnswer(ctx, next.ID, u.ID, next.Answer)
		require.NoError(t, err)
	}

	// Now the browser: it must pick up at task 4 of the same set.
	rr := env.get(t, "/tasks", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "из 10")

	tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, currentTaskID(t, rr.Body.String())))
	require.NoError(t, err)
	require.Equal(t, ds.ID, tk.DailySetID, "the browser must continue the bot's set, not open a new one")
	require.Equal(t, 3, tk.Position, "positions are 0-based, so task 4 sits at index 3")
}

// The same round open in both channels: whoever answers second must not get an
// error page, and must not be scored twice.
func TestAnsweringTwiceIsHandledGracefully(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 106, user.A1)
	ctx := context.Background()

	body := env.get(t, "/tasks", cookie).Body.String()
	id := currentTaskID(t, body)

	tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, id))
	require.NoError(t, err)

	first := env.postForm(t, "/tasks/"+id+"/answer", answerForm(tk.Answer), cookie)
	require.Equal(t, http.StatusOK, first.Code)

	second := env.postForm(t, "/tasks/"+id+"/answer", answerForm(tk.Answer), cookie)
	require.Equal(t, http.StatusNoContent, second.Code,
		"a stale answer should refresh the view, not fail")
	require.Equal(t, "true", second.Header().Get("HX-Refresh"))

	var results int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM task_results WHERE user_id = $1`, u.ID).Scan(&results))
	require.Equal(t, 1, results, "the second answer must not create a second result row")
}

func TestTasksRoundLimitIsShared(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 107, user.A1)
	ctx := context.Background()

	groupSvc := group.NewService(postgres.NewGroupRepository(sqlc.New(env.pool)))
	g, err := groupSvc.Create(ctx, "Соло", u.ID)
	require.NoError(t, err)

	// Burn all five rounds through the domain, as the bot would.
	for round := 0; round < 5; round++ {
		ds, err := env.taskSvc.GenerateDaily(ctx, u.ID, g.ID, user.A1)
		require.NoError(t, err)
		for {
			next, err := env.taskSvc.GetNextUnanswered(ctx, ds.ID, u.ID)
			require.NoError(t, err)
			if next == nil {
				break
			}
			_, err = env.taskSvc.SubmitAnswer(ctx, next.ID, u.ID, next.Answer)
			require.NoError(t, err)
		}
	}

	rr := env.get(t, "/tasks", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "Раунды на сегодня закончились",
		"the daily limit counts rounds from both channels")
}

// --- helpers -------------------------------------------------------------

// taskIDFrom parses the id lifted out of the rendered slot.
func taskIDFrom(t *testing.T, s string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(s, 10, 64)
	require.NoError(t, err)
	return id
}

func answerForm(answer string) string {
	return url.Values{"answer": {answer}}.Encode()
}

// --- assessment ----------------------------------------------------------

// currentOptionValues lifts the option indexes out of a rendered assessment slot.
func assessOptionCount(t *testing.T, body string) int {
	t.Helper()
	return strings.Count(body, `name="option"`)
}

func TestAssessRunsThirtyQuestionsAndSetsLevel(t *testing.T) {
	env := setupDBEnv(t)
	ctx := context.Background()

	u, err := env.userSvc.Register(ctx, 200, "carol", "Вера")
	require.NoError(t, err)
	require.False(t, u.Onboarded, "a fresh user has no level yet")

	token, err := env.jwt.Create(u.ID)
	require.NoError(t, err)
	cookie := &http.Cookie{Name: authCookie, Value: token}

	rr := env.get(t, "/assess", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "из 30", "the run is thirty questions")
	require.Equal(t, 4, assessOptionCount(t, rr.Body.String()))

	answered := 0
	body := rr.Body.String()
	for answered < 40 { // safety bound
		if !strings.Contains(body, `name="option"`) {
			break
		}
		// Always pick the first option: the point is the flow, and a mix of
		// right and wrong answers still terminates at exactly 30.
		fb := env.postForm(t, "/assess/answer", url.Values{"option": {"0"}}.Encode(), cookie)
		require.Equal(t, http.StatusOK, fb.Code)
		answered++
		body = fb.Body.String()
	}

	require.Equal(t, 30, answered, "the run must end after exactly thirty answers")
	require.Contains(t, body, "ваш уровень")
	require.Contains(t, body, "Разбор по вопросам", "the result must justify the level")

	u, err = env.userSvc.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.True(t, u.Onboarded, "finishing the assessment must onboard the user")
	require.NotEmpty(t, u.Level)
}

func TestAssessSkipIsRecordedAsSkip(t *testing.T) {
	env := setupDBEnv(t)
	ctx := context.Background()

	u, err := env.userSvc.Register(ctx, 201, "dave", "Дмитрий")
	require.NoError(t, err)
	token, err := env.jwt.Create(u.ID)
	require.NoError(t, err)
	cookie := &http.Cookie{Name: authCookie, Value: token}

	env.get(t, "/assess", cookie)

	// "Не знаю" posts no option at all.
	fb := env.postForm(t, "/assess/answer", url.Values{"skip": {"1"}}.Encode(), cookie)
	require.Equal(t, http.StatusOK, fb.Code)

	var skipped bool
	require.NoError(t, env.pool.QueryRow(ctx, `
		SELECT a.skipped FROM assessment_answers a
		JOIN assessment_sessions s ON s.id = a.session_id
		WHERE s.user_id = $1`, u.ID).Scan(&skipped))
	require.True(t, skipped, "an honest skip must be stored as one, not as a wrong guess")
}

// A run begun in the bot continues in the browser at the same question number.
func TestAssessStartedInBotResumesInBrowser(t *testing.T) {
	env := setupDBEnv(t)
	ctx := context.Background()

	u, err := env.userSvc.Register(ctx, 202, "erin", "Елена")
	require.NoError(t, err)
	token, err := env.jwt.Create(u.ID)
	require.NoError(t, err)
	cookie := &http.Cookie{Name: authCookie, Value: token}

	// "The bot": start and answer five questions through the domain service.
	_, _, err = env.assessSvc.Start(ctx, u.ID)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, _, _, err := env.assessSvc.SubmitAnswer(ctx, u.ID, 0)
		require.NoError(t, err)
	}

	rr := env.get(t, "/assess", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	// Five answered means the browser opens question six of the same session.
	require.Contains(t, rr.Body.String(), ">0<", "the counter is rendered as flaps")
	require.Contains(t, rr.Body.String(), ">6<", "the browser must continue at question six")

	var sessions int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM assessment_sessions WHERE user_id = $1`, u.ID).Scan(&sessions))
	require.Equal(t, 1, sessions, "the browser must resume the session, not open a second one")
}

// --- words ---------------------------------------------------------------

// currentWordSlot lifts the word id and mode out of a rendered drill.
func currentWordSlot(t *testing.T, body string) (id string, mode string) {
	t.Helper()
	const marker = `hx-post="/words/`
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "no word form in the rendered slot")
	rest := body[i+len(marker):]
	id = rest[:strings.Index(rest, "/answer")]

	const mm = `name="mode" value="`
	j := strings.Index(body, mm)
	require.GreaterOrEqual(t, j, 0, "slot carries no mode")
	rest = body[j+len(mm):]
	return id, rest[:strings.Index(rest, `"`)]
}

func TestWordsOpensSessionWithTenWords(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 300, user.A1)

	rr := env.get(t, "/words", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "слов в сессии")
	require.Contains(t, rr.Body.String(), "по уровням")

	session, err := env.wordsSvc.RepoOpenSession(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Len(t, session.WordIDs, 10, "a session is ten words")
}

func TestWordsCorrectAnswerAdvancesModeCounter(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 301, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	id, mode := currentWordSlot(t, body)
	wordID := taskIDFrom(t, id)

	word, err := env.srsRepo.GetWordByID(ctx, wordID)
	require.NoError(t, err)
	expected := words.ExpectedAnswer(word, words.Mode(mode))

	fb := env.postForm(t, "/words/"+id+"/answer",
		url.Values{"mode": {mode}, "answer": {expected}}.Encode(), cookie)
	require.Equal(t, http.StatusOK, fb.Code)
	require.Contains(t, fb.Body.String(), "Верно")

	progress, err := env.wordsSvc.RepoGetProgress(ctx, u.ID, wordID)
	require.NoError(t, err)
	require.Equal(t, 1, progress.Counts[words.Mode(mode)], "one correct answer counts once")
}

// A wrong answer resets the counter for that mode — that is the core of the
// drill, and it is easy to break without noticing.
func TestWordsWrongAnswerResetsModeCounter(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 302, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	id, mode := currentWordSlot(t, body)
	wordID := taskIDFrom(t, id)

	word, err := env.srsRepo.GetWordByID(ctx, wordID)
	require.NoError(t, err)
	expected := words.ExpectedAnswer(word, words.Mode(mode))

	env.postForm(t, "/words/"+id+"/answer",
		url.Values{"mode": {mode}, "answer": {expected}}.Encode(), cookie)

	fb := env.postForm(t, "/words/"+id+"/answer",
		url.Values{"mode": {mode}, "answer": {"заведомо неверно"}}.Encode(), cookie)
	require.Contains(t, fb.Body.String(), "Неверно")

	progress, err := env.wordsSvc.RepoGetProgress(ctx, u.ID, wordID)
	require.NoError(t, err)
	require.Equal(t, 0, progress.Counts[words.Mode(mode)], "a wrong answer must reset the mode")
}

// "Уже знаю это слово" is the fast path for a B2 speaker meeting A1 words: it
// retires the word and drops it straight into spaced repetition.
func TestWordsAlreadyKnownRetiresWordIntoSRS(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 303, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	id, _ := currentWordSlot(t, body)
	wordID := taskIDFrom(t, id)

	rr := env.postForm(t, "/words/"+id+"/know", "", cookie)
	require.Equal(t, http.StatusOK, rr.Code)

	progress, err := env.wordsSvc.RepoGetProgress(ctx, u.ID, wordID)
	require.NoError(t, err)
	require.True(t, progress.Learned, "the word must be marked learned")

	var cards int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM review_cards WHERE user_id = $1 AND word_id = $2`,
		u.ID, wordID).Scan(&cards))
	require.Equal(t, 1, cards, "a retired word must enter spaced repetition")
}

// The defer affordance only appears once a word has really stopped sticking —
// showing it earlier would invite skipping anything mildly hard.
func TestWordsDeferOfferedOnlyAfterThreshold(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 304, user.A1)

	body := env.get(t, "/words", cookie).Body.String()
	id, mode := currentWordSlot(t, body)

	var lastBody string
	for i := 0; i <= words.DeferThreshold; i++ {
		fb := env.postForm(t, "/words/"+id+"/answer",
			url.Values{"mode": {mode}, "answer": {"заведомо неверно"}}.Encode(), cookie)
		require.Equal(t, http.StatusOK, fb.Code)
		lastBody = fb.Body.String()
		if i < words.DeferThreshold {
			require.NotContains(t, lastBody, "Отложить до завтра",
				"defer must not be offered at mistake %d", i+1)
		}
	}
	require.Contains(t, lastBody, "Отложить до завтра",
		"defer must appear past the threshold")
}

func TestWordsDeferRemovesWordFromSession(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 305, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	id, _ := currentWordSlot(t, body)
	wordID := taskIDFrom(t, id)

	rr := env.postForm(t, "/words/"+id+"/defer", "", cookie)
	require.Equal(t, http.StatusOK, rr.Code)

	session, err := env.wordsSvc.RepoOpenSession(ctx, u.ID)
	require.NoError(t, err)
	require.True(t, session.IsDeferred(wordID), "the word must be excluded from the rest of the session")
	require.Len(t, session.WordIDs, 10, "deferring keeps the pick intact, it only skips the word")
}

// /words must not touch the leaderboard — that is its one behavioural
// difference from /tasks, and it is invisible in the UI.
func TestWordsDoNotScore(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 306, user.A1)
	ctx := context.Background()

	body := env.get(t, "/words", cookie).Body.String()
	id, mode := currentWordSlot(t, body)
	word, err := env.srsRepo.GetWordByID(ctx, taskIDFrom(t, id))
	require.NoError(t, err)

	env.postForm(t, "/words/"+id+"/answer",
		url.Values{"mode": {mode}, "answer": {words.ExpectedAnswer(word, words.Mode(mode))}}.Encode(), cookie)

	var rows int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM scores WHERE user_id = $1`, u.ID).Scan(&rows))
	require.Zero(t, rows, "/words must never write a score row")
}

// A session opened in the bot is the same session the browser continues: the
// partial index guarantees one open session per user.
func TestWordsSessionSharedWithBot(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 307, user.A1)
	ctx := context.Background()

	botSession, _, _, err := env.wordsSvc.StartOrResumeSession(ctx, u.ID)
	require.NoError(t, err)

	env.get(t, "/words", cookie)

	webSession, err := env.wordsSvc.RepoOpenSession(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, botSession.ID, webSession.ID, "the browser must continue the bot's session")

	var sessions int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM words_sessions WHERE user_id = $1 AND completed_at IS NULL`,
		u.ID).Scan(&sessions))
	require.Equal(t, 1, sessions)
}

// A "type it in English" task must not display the English word — that would
// hand over the answer, and the transcription would hand over the
// pronunciation with it. Same for the RU→EN choice task.
func TestVocabTasksNeverShowTheAnswerSide(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 108, user.A1)
	ctx := context.Background()

	checked := map[string]bool{}
	for i := 0; i < 40 && len(checked) < 2; i++ {
		body := env.get(t, "/tasks", cookie).Body.String()
		if !strings.Contains(body, `hx-post="/tasks/`) {
			break
		}
		id := currentTaskID(t, body)
		tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)

		subtype, _ := tk.Metadata["subtype"].(string)
		if subtype == task.VocabSubtypeRuEn || subtype == task.VocabSubtypeTypingRuEn {
			word, err := env.srsRepo.GetWordByID(ctx, int64(tk.Metadata["word_id"].(float64)))
			require.NoError(t, err)

			// The prompt area must carry the Russian side, not the English one.
			require.Contains(t, body, word.Russian,
				"%s must prompt with the Russian word", subtype)
			if subtype == task.VocabSubtypeTypingRuEn {
				require.NotContains(t, body, ">"+word.English+"<",
					"%s must not display the English answer", subtype)
				if word.Transcription != "" {
					require.NotContains(t, body, word.Transcription,
						"%s must not display the transcription of the answer", subtype)
				}
			}
			checked[subtype] = true
		}

		// Move past this task so the next iteration sees a different one.
		_, err = env.taskSvc.SubmitSkip(ctx, tk.ID, mustUserID(t, env, 108))
		require.NoError(t, err)
	}

	require.NotEmpty(t, checked, "no RU→EN vocab task appeared in 40 draws")
}

func mustUserID(t *testing.T, env *dbEnv, telegramID int64) int64 {
	t.Helper()
	u, err := env.userSvc.GetByTelegramID(context.Background(), telegramID)
	require.NoError(t, err)
	return u.ID
}

// --- learn ---------------------------------------------------------------

func TestLearnListsTopicsForUserLevelAndNext(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 400, user.A1)

	rr := env.get(t, "/learn", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	require.Contains(t, body, "Уровень A1")
	require.Contains(t, body, "Уровень A2", "the next level is offered so the user can look ahead")
	require.NotContains(t, body, "Уровень C2", "levels far above the user must not be listed")
	require.Contains(t, body, "/learn/topic?name=")
}

func TestLearnTopicShowsTheoryAndStart(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 401, user.A1)

	body := env.get(t, "/learn", cookie).Body.String()
	topic := firstTopic(t, body)

	rr := env.get(t, "/learn/topic?name="+url.QueryEscape(topic), cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "Начать практику")
	// Форма слова зависит от числа (1 вопрос / 3 вопроса / 5 вопросов),
	// поэтому проверяем корень, а не конкретное окончание.
	require.Contains(t, rr.Body.String(), "вопрос")
}

// The run is stateless: seed plus index reproduce the sample, so the same URL
// always yields the same question. Without that a refresh would reshuffle
// mid-run and the user would lose their place.
func TestLearnPracticeIsReproducibleFromSeed(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 402, user.A1)

	body := env.get(t, "/learn", cookie).Body.String()
	topic := url.QueryEscape(firstTopic(t, body))

	first := env.get(t, "/learn/practice?name="+topic+"&seed=12345&i=0", cookie)
	require.Equal(t, http.StatusOK, first.Code)
	again := env.get(t, "/learn/practice?name="+topic+"&seed=12345&i=0", cookie)
	require.Equal(t, first.Body.String(), again.Body.String(),
		"the same seed and index must yield the same question")

	other := env.get(t, "/learn/practice?name="+topic+"&seed=999&i=0", cookie)
	require.Equal(t, http.StatusOK, other.Code)
}

func TestLearnAnswerReportsCorrectness(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 403, user.A1)
	ctx := context.Background()

	body := env.get(t, "/learn", cookie).Body.String()
	topic := firstTopic(t, body)

	questions, err := env.grammarRepo.GetQuestionsByCategory(ctx, topic)
	require.NoError(t, err)
	sample := task.StratifiedSample(questions, 12345, 20)
	require.NotEmpty(t, sample)

	form := url.Values{
		"name": {topic}, "seed": {"12345"}, "i": {"0"},
		"answer": {sample[0].Answer}, "run": {"0"},
	}
	rr := env.postForm(t, "/learn/answer", form.Encode(), cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "Верно")

	form.Set("answer", "заведомо неверно")
	rr = env.postForm(t, "/learn/answer", form.Encode(), cookie)
	require.Contains(t, rr.Body.String(), "Неверно")
	require.Contains(t, rr.Body.String(), sample[0].Answer)

	// Practice must not touch scores or the SRS — it is a drill, not a task.
	var scores, cards int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM scores WHERE user_id = $1`, u.ID).Scan(&scores))
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT count(*) FROM review_cards WHERE user_id = $1`, u.ID).Scan(&cards))
	require.Zero(t, scores, "/learn must not write scores")
	require.Zero(t, cards, "/learn must not write review cards")
}

func TestLearnUnknownTopicIs404(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 404, user.A1)

	rr := env.get(t, "/learn/topic?name="+url.QueryEscape("нет такой темы"), cookie)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

// firstTopic pulls a category name out of the rendered index.
func firstTopic(t *testing.T, body string) string {
	t.Helper()
	const marker = `/learn/topic?name=`
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "index lists no topics")
	rest := body[i+len(marker):]
	raw := rest[:strings.Index(rest, `"`)]
	decoded, err := url.QueryUnescape(strings.ReplaceAll(raw, "&amp;", "&"))
	require.NoError(t, err)
	return decoded
}

// The rule is shown when the user got it wrong — that is the moment it helps.
// After a correct answer it is noise, and they read the theory a screen ago.
func TestLearnExplanationOnlyOnWrongAnswer(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 405, user.A1)
	ctx := context.Background()

	topic := firstTopic(t, env.get(t, "/learn", cookie).Body.String())
	questions, err := env.grammarRepo.GetQuestionsByCategory(ctx, topic)
	require.NoError(t, err)
	sample := task.StratifiedSample(questions, 777, 20)
	require.NotEmpty(t, sample)
	if sample[0].Explanation == "" {
		t.Skip("this topic carries no explanation")
	}

	form := url.Values{"name": {topic}, "seed": {"777"}, "i": {"0"}, "run": {"0"}}

	form.Set("answer", sample[0].Answer)
	right := env.postForm(t, "/learn/answer", form.Encode(), cookie).Body.String()

	form.Set("answer", "заведомо неверно")
	wrong := env.postForm(t, "/learn/answer", form.Encode(), cookie).Body.String()

	snippet := sample[0].Explanation
	if len(snippet) > 40 {
		snippet = snippet[:40]
	}
	require.Contains(t, wrong, snippet, "a wrong answer must be explained")
	require.NotContains(t, right, snippet, "a correct answer must not repeat the theory")
}

// --- leaderboard, profile, groups ----------------------------------------

func TestProfileShowsVocabularyAndStreak(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 500, user.B1)

	rr := env.get(t, "/profile", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	require.Contains(t, body, "В интервальных повторениях")
	require.Contains(t, body, "Усвоено в разделе «Слова»")
	require.Contains(t, body, "дней подряд")
	// A zero streak needs explaining, or it reads as the app losing history.
	require.Contains(t, body, "Серия считает и сегодняшний день")
}

func TestLeaderboardWithoutGroupsInvitesToCreateOne(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 501, user.A1)

	rr := env.get(t, "/leaderboard", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "Пока не с кем соревноваться")
	require.Contains(t, rr.Body.String(), "/groups")
}

func TestGroupCreateAndJoinByCode(t *testing.T) {
	env := setupDBEnv(t)
	_, ownerCookie := env.signedIn(t, 502, user.A1)
	_, joinerCookie := env.signedIn(t, 503, user.A1)

	created := env.postForm(t, "/groups/create",
		url.Values{"name": {"English Club"}}.Encode(), ownerCookie)
	require.Equal(t, http.StatusOK, created.Code)
	require.Contains(t, created.Body.String(), "English Club")

	code := inviteCodeFrom(t, created.Body.String())

	joined := env.postForm(t, "/groups/join",
		url.Values{"code": {code}}.Encode(), joinerCookie)
	require.Equal(t, http.StatusOK, joined.Code)
	require.Contains(t, joined.Body.String(), "вступили в группу")

	// Joining twice is a mistake, not a failure — say so plainly.
	again := env.postForm(t, "/groups/join",
		url.Values{"code": {code}}.Encode(), joinerCookie)
	require.Contains(t, again.Body.String(), "уже состоите")
}

// A wrong code and a nonexistent group look identical to the user: telling
// them apart would let someone probe which codes exist.
func TestGroupJoinWithBadCodeSaysNotFound(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 504, user.A1)

	rr := env.postForm(t, "/groups/join", url.Values{"code": {"deadbeef"}}.Encode(), cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "не найдена")
}

func TestGroupCreateRejectsEmptyName(t *testing.T) {
	env := setupDBEnv(t)
	_, cookie := env.signedIn(t, 505, user.A1)

	rr := env.postForm(t, "/groups/create", url.Values{"name": {"   "}}.Encode(), cookie)
	require.Contains(t, rr.Body.String(), "Введите название")
}

// Passing another group's id must not expose its standings.
func TestLeaderboardIgnoresGroupTheUserIsNotIn(t *testing.T) {
	env := setupDBEnv(t)
	_, ownerCookie := env.signedIn(t, 506, user.A1)
	outsider, outsiderCookie := env.signedIn(t, 507, user.A1)
	ctx := context.Background()

	env.postForm(t, "/groups/create", url.Values{"name": {"Private"}}.Encode(), ownerCookie)
	env.postForm(t, "/groups/create", url.Values{"name": {"Outsider Own"}}.Encode(), outsiderCookie)

	var privateID int64
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT id FROM groups WHERE name = 'Private'`).Scan(&privateID))

	rr := env.get(t, "/leaderboard?group="+strconv.FormatInt(privateID, 10), outsiderCookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "Outsider Own",
		"an id the user is not a member of must fall back to their own group")
	require.NotContains(t, rr.Body.String(), "Private")
	_ = outsider
}

func TestLeaderboardRanksAnsweredTasks(t *testing.T) {
	env := setupDBEnv(t)
	u, cookie := env.signedIn(t, 508, user.A1)
	ctx := context.Background()

	// Answer a full round so there is something to rank.
	body := env.get(t, "/tasks", cookie).Body.String()
	for i := 0; i < 10; i++ {
		if !strings.Contains(body, `hx-post="/tasks/`) {
			break
		}
		id := currentTaskID(t, body)
		tk, err := env.taskSvc.GetTaskByID(ctx, taskIDFrom(t, id))
		require.NoError(t, err)
		env.postForm(t, "/tasks/"+id+"/answer", answerForm(tk.Answer), cookie)
		body = env.get(t, "/tasks/next", cookie).Body.String()
	}

	rr := env.get(t, "/leaderboard", cookie)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "— вы", "the user must be marked in their own standings")

	var raw int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT coalesce(sum(raw),0) FROM scores WHERE user_id = $1 AND period = 'day'`,
		u.ID).Scan(&raw))
	require.Greater(t, raw, 0)
}

func inviteCodeFrom(t *testing.T, body string) string {
	t.Helper()
	const marker = `Инвайт-код: <span class="font-mono text-lg">`
	i := strings.Index(body, marker)
	require.GreaterOrEqual(t, i, 0, "no invite code in the response")
	rest := body[i+len(marker):]
	return rest[:strings.Index(rest, "<")]
}
