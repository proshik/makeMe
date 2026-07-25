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
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
)

// dbEnv is a full stack over a real Postgres: the same services the bot uses,
// plus the web router on top. No WireMock here — the web adapter never calls
// the Telegram API.
type dbEnv struct {
	router    *chi.Mux
	jwt       *JWTService
	userSvc   *user.Service
	taskSvc   *task.Service
	assessSvc *assessment.Service
	pool      *pgxpool.Pool
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

	jwtSvc, err := NewJWTService(testSecret(t), 24)
	require.NoError(t, err)

	// The OIDC endpoint is never reached in these tests — the session cookie is
	// minted directly.
	srv, err := New(Deps{
		UserSvc: userSvc, GroupSvc: groupSvc, TaskSvc: taskSvc,
		ScoringSvc: scoringSvc, SRSRepo: srsRepo, AssessSvc: assessSvc,
	}, jwtSvc, NewOIDCClient("http://unused", "client-1", "secret", "http://app/cb"), false)
	require.NoError(t, err)

	r := chi.NewRouter()
	srv.Mount(r)

	return &dbEnv{router: r, jwt: jwtSvc, userSvc: userSvc, taskSvc: taskSvc,
		assessSvc: assessSvc, pool: pool}
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
