package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"makeMe/internal/config"
	"makeMe/internal/dictionary"
	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/server"
	"makeMe/internal/storage/postgres"
	"makeMe/internal/storage/sqlc"
	"makeMe/internal/telegram"
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

	if err := seedWords(ctx, cfg.DatabaseURL, cfg.WordsFile); err != nil {
		slog.Warn("word seed skipped", "error", err)
	}

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

	assessRepo := postgres.NewAssessmentRepository(queries)
	assessSvc := assessment.NewService(assessRepo, userSvc, srsRepo)

	taskRepo := postgres.NewTaskRepository(queries)
	vocabGen := task.NewVocabGenerator(srsRepo)
	grammarRepo := postgres.NewGrammarRepository(queries)
	grammarGen := task.NewGrammarGenerator(grammarRepo)
	taskSvc := task.NewService(taskRepo, srsRepo, vocabGen, grammarGen)

	scoringRepo := postgres.NewScoringRepository(queries)
	scoringSvc := scoring.NewService(scoringRepo)

	dictClient := dictionary.NewClient()

	tgBot, err := telegram.New(cfg.TelegramToken, userSvc, groupSvc, assessSvc, taskSvc, scoringSvc, srsRepo, dictClient)
	if err != nil {
		return err
	}

	if err := tgBot.SetWebhook(ctx, cfg.WebhookURL); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tgBot.StartWebhook(ctx)
	}()

	r := server.New(tgBot)

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

type wordEntry struct {
	English       string `json:"english"`
	Russian       string `json:"russian"`
	Level         string `json:"level"`
	Example       string `json:"example"`
	Transcription string `json:"transcription"`
}

func seedWords(ctx context.Context, dbURL, wordsFile string) error {
	data, err := os.ReadFile(wordsFile)
	if err != nil {
		return fmt.Errorf("read words file: %w", err)
	}

	var words []wordEntry
	if err := json.Unmarshal(data, &words); err != nil {
		return fmt.Errorf("parse words file: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect for seed: %w", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)
	for _, w := range words {
		if err := q.UpsertWord(ctx, sqlc.UpsertWordParams{
			English:       w.English,
			Russian:       w.Russian,
			Level:         w.Level,
			Example:       w.Example,
			Transcription: w.Transcription,
		}); err != nil {
			slog.Warn("failed to upsert word", "word", w.English, "error", err)
		}
	}
	slog.Info("words seeded", "count", len(words))
	return nil
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
}
