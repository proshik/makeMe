package telegram_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	wiremocktc "github.com/wiremock/wiremock-testcontainers-go"

	_ "github.com/jackc/pgx/v5/stdlib"

	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
	"makeMe/internal/telegram"
)

type testEnv struct {
	pool        *pgxpool.Pool
	queries     *sqlc.Queries
	userRepo    *postgres.UserRepository
	assessRepo  *postgres.AssessmentRepository
	userSvc     *user.Service
	groupSvc    *group.Service
	assessSvc   *assessment.Service
	taskSvc     *task.Service
	scoringSvc  *scoring.Service
	srsRepo     *postgres.SRSRepository
	wordsSvc    *words.Service
	wordsRepo   *postgres.WordsRepository
	pendingRepo *postgres.PendingTypingRepository
	tgBot       *telegram.Bot
	webhookURL  string
	wiremockURL string
	httpServer  *httptest.Server
}

func setupTestEnv(t *testing.T) *testEnv {
	return setupTestEnvWithMaxRounds(t, 5)
}

func setupTestEnvWithMaxRounds(t *testing.T, maxRounds int) *testEnv {
	t.Helper()
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:17",
		tcpostgres.WithDatabase("makeme_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
		testcontainersWithStartupTimeout(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	dbURL, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run migrations
	runMigrations(t, dbURL)

	// Create pgx pool
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	// Start WireMock container
	wmContainer, err := wiremocktc.RunContainerAndStopOnCleanup(ctx, t,
		testcontainers.WithImage("wiremock/wiremock:3.13.0"),
	)
	require.NoError(t, err)

	wiremockURL, err := wiremocktc.GetURI(ctx, wmContainer.Container)
	require.NoError(t, err)

	// Stub getMe (called during bot initialization if not skipped)
	stubGetMe(t, wiremockURL)
	// Stub sendMessage (called by all handlers)
	stubSendMessage(t, wiremockURL)
	// Stub answerCallbackQuery (called by callback handlers)
	stubAnswerCallbackQuery(t, wiremockURL)
	// Stub sendAudio (called by word audio handler)
	stubSendAudio(t, wiremockURL)

	// Create services
	queries := sqlc.New(pool)

	userRepo := postgres.NewUserRepository(queries)
	userSvc := user.NewService(userRepo)

	groupRepo := postgres.NewGroupRepository(queries)
	groupSvc := group.NewService(groupRepo)

	srsRepo := postgres.NewSRSRepository(queries)

	grammarRepo := postgres.NewGrammarRepository(queries)
	grammarGen := task.NewGrammarGenerator(grammarRepo)

	assessRepo := postgres.NewAssessmentRepository(queries)
	assessSvc := assessment.NewService(assessRepo, userSvc, srsRepo, grammarRepo)

	taskRepo := postgres.NewTaskRepository(queries)
	vocabGen := task.NewVocabGenerator(srsRepo)
	taskSvc := task.NewService(taskRepo, srsRepo, maxRounds, vocabGen, grammarGen)

	scoringRepo := postgres.NewScoringRepository(queries)
	scoringSvc := scoring.NewService(scoringRepo)

	wordsRepo := postgres.NewWordsRepository(queries)
	pendingRepo := postgres.NewPendingTypingRepository(queries)
	wordsSvc := words.NewService(wordsRepo, srsRepo)

	// Use nil dictClient so tests never call the real external dictionary API.
	// The handler already guards on dictClient != nil and falls back to "not found".
	// Create bot with WireMock as Telegram API
	tgBot, err := telegram.New("test-token", userSvc, groupSvc, assessSvc, taskSvc, scoringSvc, srsRepo, nil, grammarRepo, wordsSvc, pendingRepo,
		bot.WithServerURL(wiremockURL),
	)
	require.NoError(t, err)

	go tgBot.StartWebhook(ctx)

	// Create test HTTP server with webhook handler
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", tgBot.WebhookHandler())
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	return &testEnv{
		pool:        pool,
		queries:     queries,
		userRepo:    userRepo,
		assessRepo:  assessRepo,
		userSvc:     userSvc,
		groupSvc:    groupSvc,
		assessSvc:   assessSvc,
		taskSvc:     taskSvc,
		scoringSvc:  scoringSvc,
		srsRepo:     srsRepo,
		wordsSvc:    wordsSvc,
		wordsRepo:   wordsRepo,
		pendingRepo: pendingRepo,
		tgBot:       tgBot,
		webhookURL:  srv.URL + "/webhook",
		wiremockURL: wiremockURL,
		httpServer:  srv,
	}
}

func testcontainersWithStartupTimeout() testcontainers.ContainerCustomizer {
	return testcontainers.WithWaitStrategy(
		wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	)
}

func runMigrations(t *testing.T, dbURL string) {
	t.Helper()

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	defer db.Close()

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	err = goose.Up(db, migrationsDir)
	require.NoError(t, err)
}

func stubGetMe(t *testing.T, wiremockURL string) {
	t.Helper()
	body := `{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`
	stubJSON(t, wiremockURL, "/bottest-token/getMe", body)
}

func stubSendMessage(t *testing.T, wiremockURL string) {
	t.Helper()
	body := `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":0,"type":"private"}}}`
	stubAny(t, wiremockURL, "/bottest-token/sendMessage", body)
}

func stubAnswerCallbackQuery(t *testing.T, wiremockURL string) {
	t.Helper()
	body := `{"ok":true,"result":true}`
	stubAny(t, wiremockURL, "/bottest-token/answerCallbackQuery", body)
}

func stubSendAudio(t *testing.T, wiremockURL string) {
	t.Helper()
	body := `{"ok":true,"result":{"message_id":2,"date":0,"chat":{"id":0,"type":"private"}}}`
	stubAny(t, wiremockURL, "/bottest-token/sendAudio", body)
}

func stubJSON(t *testing.T, wiremockURL, urlPath, responseBody string) {
	t.Helper()
	mapping := fmt.Sprintf(`{
		"request": {"method": "POST", "urlPath": "%s"},
		"response": {"status": 200, "body": %q, "headers": {"Content-Type": "application/json"}}
	}`, urlPath, responseBody)

	req, err := http.NewRequest("POST", wiremockURL+"/__admin/mappings", bytes.NewBufferString(mapping))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func stubAny(t *testing.T, wiremockURL, urlPath, responseBody string) {
	t.Helper()
	mapping := fmt.Sprintf(`{
		"request": {"method": "ANY", "urlPath": "%s"},
		"response": {"status": 200, "body": %q, "headers": {"Content-Type": "application/json"}}
	}`, urlPath, responseBody)

	req, err := http.NewRequest("POST", wiremockURL+"/__admin/mappings", bytes.NewBufferString(mapping))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

// sendUpdate sends a Telegram update to the webhook and waits for the bot
// to fully process it. Handlers often make multiple sequential Telegram API
// calls (e.g. answerCallbackQuery + sendMessage), so we wait until the
// WireMock request count stops growing rather than just increasing once.
func (e *testEnv) sendUpdate(t *testing.T, update models.Update) {
	t.Helper()

	const (
		pollInterval = 50 * time.Millisecond
		stableNeeded = 3 // consecutive stable polls before we consider processing done
		totalTimeout = 10 * time.Second
	)

	initialCount := e.countAllWireMockRequests(t)

	body, err := json.Marshal(update)
	require.NoError(t, err)

	resp, err := http.Post(e.webhookURL, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Poll until the request count has risen above initialCount AND remained
	// stable for stableNeeded consecutive polls (~150 ms). This ensures all
	// API calls made by the handler (answerCallbackQuery, sendMessage, etc.)
	// are recorded in WireMock before we return.
	stableRuns := 0
	prevCount := initialCount
	require.Eventually(t, func() bool {
		current := e.countAllWireMockRequests(t)
		if current <= initialCount {
			stableRuns = 0
			prevCount = current
			return false
		}
		if current != prevCount {
			stableRuns = 1
			prevCount = current
			return false
		}
		stableRuns++
		return stableRuns >= stableNeeded
	}, totalTimeout, pollInterval, "timeout waiting for bot to finish processing the update")
}

// countAllWireMockRequests returns the number of requests recorded by WireMock
// by counting the entries in the requests array.
func (e *testEnv) countAllWireMockRequests(t *testing.T) int {
	t.Helper()

	req, err := http.NewRequest("GET", e.wiremockURL+"/__admin/requests", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Requests []json.RawMessage `json:"requests"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return len(result.Requests)
}

// getSendMessageRequests returns all sendMessage calls captured by WireMock.
func (e *testEnv) getSendMessageRequests(t *testing.T) []wiremockRequest {
	t.Helper()
	return e.getRequests(t, "/bottest-token/sendMessage")
}

func (e *testEnv) getRequests(t *testing.T, urlPath string) []wiremockRequest {
	t.Helper()

	reqBody := fmt.Sprintf(`{"method":"POST","urlPath":"%s"}`, urlPath)
	req, err := http.NewRequest("POST", e.wiremockURL+"/__admin/requests/find", bytes.NewBufferString(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Requests []wiremockRequest `json:"requests"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	return result.Requests
}

// resetWireMock clears all recorded requests.
func (e *testEnv) resetWireMock(t *testing.T) {
	t.Helper()
	req, err := http.NewRequest("POST", e.wiremockURL+"/__admin/requests/reset", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
}

type wiremockRequest struct {
	Body string `json:"body"`
}

// makeCallbackUpdate creates a models.Update simulating an inline keyboard callback.
func makeCallbackUpdate(updateID int64, userID int64, chatID int64, username, firstName, callbackData string) models.Update {
	return models.Update{
		ID: updateID,
		CallbackQuery: &models.CallbackQuery{
			ID: fmt.Sprintf("cb_%d", updateID),
			From: models.User{
				ID:        userID,
				Username:  username,
				FirstName: firstName,
				IsBot:     false,
			},
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   1,
					Date: int(time.Now().Unix()),
					Chat: models.Chat{ID: chatID, Type: "private"},
				},
			},
			Data: callbackData,
		},
	}
}

// makeTextUpdate creates a models.Update simulating a text message from a user.
func makeTextUpdate(updateID int64, userID int64, chatID int64, username, firstName, text string) models.Update {
	return models.Update{
		ID: updateID,
		Message: &models.Message{
			ID:   1,
			Date: int(time.Now().Unix()),
			Chat: models.Chat{ID: chatID, Type: "private"},
			From: &models.User{
				ID:        userID,
				Username:  username,
				FirstName: firstName,
				IsBot:     false,
			},
			Text: text,
		},
	}
}
