package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"makeMe/internal/config"
	"makeMe/internal/dictionary"
	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
	"makeMe/internal/scheduler"
	"makeMe/internal/seed"
	"makeMe/internal/server"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
	"makeMe/internal/telegram"
	"makeMe/internal/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	setupLogger(cfg.LogLevel)

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		return err
	}

	seed.All(ctx, cfg.DatabaseURL, cfg.WordsFile, cfg.GrammarCategoriesFile, cfg.GrammarQuestionsFile)

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("connected to database")

	queries := sqlc.New(pool)

	userRepo := postgres.NewUserRepository(queries)
	userSvc := user.NewService(userRepo)

	groupRepo := postgres.NewGroupRepository(queries)
	groupSvc := group.NewService(groupRepo)

	srsRepo := postgres.NewSRSRepository(queries)

	grammarRepo := postgres.NewGrammarRepository(queries)

	assessRepo := postgres.NewAssessmentRepository(queries)
	assessSvc := assessment.NewService(assessRepo, userSvc, srsRepo, grammarRepo)

	taskRepo := postgres.NewTaskRepository(queries)
	vocabGen := task.NewVocabGenerator(srsRepo)
	grammarGen := task.NewGrammarGenerator(grammarRepo)
	taskSvc := task.NewService(taskRepo, srsRepo, cfg.MaxRoundsPerDay, vocabGen, grammarGen)

	scoringRepo := postgres.NewScoringRepository(queries)
	scoringSvc := scoring.NewService(scoringRepo)

	dictClient := dictionary.NewClient()

	wordsRepo := postgres.NewWordsRepository(queries)
	pendingRepo := postgres.NewPendingTypingRepository(queries)
	wordsSvc := words.NewService(wordsRepo, srsRepo)

	tgBot, err := telegram.New(cfg.TelegramToken, userSvc, groupSvc, assessSvc, taskSvc, scoringSvc, srsRepo, dictClient, grammarRepo, wordsSvc, pendingRepo)
	if err != nil {
		return err
	}

	if err := tgBot.SetWebhook(ctx, cfg.WebhookURL); err != nil {
		return err
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		slog.Warn("unknown timezone, falling back to UTC", "timezone", cfg.Timezone)
		loc = time.UTC
	}

	sched := scheduler.New(userRepo, groupSvc, scoringSvc, assessRepo, tgBot.SendMessage, loc, cfg.SchedulerTime)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tgBot.StartWebhook(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sched.Run(ctx)
	}()

	webSrv, err := buildWebServer(cfg, web.Deps{
		UserSvc:     userSvc,
		GroupSvc:    groupSvc,
		AssessSvc:   assessSvc,
		TaskSvc:     taskSvc,
		ScoringSvc:  scoringSvc,
		SRSRepo:     srsRepo,
		WordsSvc:    wordsSvc,
		GrammarRepo: grammarRepo,
	})
	if err != nil {
		return err
	}

	r := server.New(tgBot, server.AdminConfig{
		Pool:       pool,
		DictClient: dictClient,
		AdminToken: cfg.AdminToken,
	}, webSrv)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("starting server", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	wg.Wait()
	return nil
}

// buildWebServer wires the browser UI. It returns (nil, nil) when the web
// credentials are absent — that is a supported deployment, not an error: the
// bot runs on its own and /login simply isn't mounted.
func buildWebServer(cfg *config.Config, deps web.Deps) (*web.Server, error) {
	if !cfg.WebEnabled() {
		slog.Warn("web UI disabled: set APP_BASE_URL, TELEGRAM_CLIENT_ID, TELEGRAM_CLIENT_SECRET and JWT_SECRET to enable it")
		return nil, nil
	}

	jwtSvc, err := web.NewJWTService(cfg.JWTSecret, cfg.JWTTTLHours)
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}

	oidc := web.NewOIDCClient(
		cfg.TelegramOAuthBaseURL,
		cfg.TelegramClientID,
		cfg.TelegramClientSecret,
		cfg.RedirectURI(),
	)

	srv, err := web.New(deps, jwtSvc, oidc, cfg.CookieSecure)
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	slog.Info("web UI enabled", "base_url", cfg.AppBaseURL, "redirect_uri", cfg.RedirectURI())
	return srv, nil
}

func runMigrations(dbURL string) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("migrations applied")
	return nil
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
	slog.Info("log level configured", "level", lvl)
}
