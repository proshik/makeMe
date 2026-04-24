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
	"makeMe/internal/scheduler"
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

	if err := seedGrammarCategories(ctx, cfg.DatabaseURL, cfg.GrammarCategoriesFile); err != nil {
		slog.Warn("grammar categories seed skipped", "error", err)
	}
	if err := seedGrammarQuestions(ctx, cfg.DatabaseURL, cfg.GrammarQuestionsFile); err != nil {
		slog.Warn("grammar questions seed skipped", "error", err)
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

	tgBot, err := telegram.New(cfg.TelegramToken, userSvc, groupSvc, assessSvc, taskSvc, scoringSvc, srsRepo, dictClient, grammarRepo)
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

type grammarCategoryEntry struct {
	Name        string `json:"name"`
	Explanation string `json:"explanation"`
}

type grammarQuestionEntry struct {
	Level       string   `json:"level"`
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory"`
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Answer      string   `json:"answer"`
}

func seedGrammarCategories(ctx context.Context, dbURL, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read grammar categories file: %w", err)
	}

	var entries []grammarCategoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse grammar categories file: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect for seed: %w", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)
	for _, e := range entries {
		if err := q.UpsertGrammarCategory(ctx, sqlc.UpsertGrammarCategoryParams{
			Name:        e.Name,
			Explanation: e.Explanation,
		}); err != nil {
			slog.Warn("failed to upsert grammar category", "name", e.Name, "error", err)
		}
	}
	slog.Info("grammar categories seeded", "count", len(entries))
	return nil
}

func seedGrammarQuestions(ctx context.Context, dbURL, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read grammar questions file: %w", err)
	}

	var entries []grammarQuestionEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse grammar questions file: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect for seed: %w", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)
	for _, e := range entries {
		optJSON, err := json.Marshal(e.Options)
		if err != nil {
			slog.Warn("failed to marshal options", "prompt", e.Prompt, "error", err)
			continue
		}
		subcategory := e.Subcategory
		if subcategory == "" {
			subcategory = "general"
		}
		if err := q.UpsertGrammarQuestion(ctx, sqlc.UpsertGrammarQuestionParams{
			Level:       e.Level,
			Category:    e.Category,
			Subcategory: subcategory,
			Prompt:      e.Prompt,
			Options:     optJSON,
			Answer:      e.Answer,
		}); err != nil {
			slog.Warn("failed to upsert grammar question", "prompt", e.Prompt, "error", err)
		}
	}
	slog.Info("grammar questions seeded", "count", len(entries))
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
