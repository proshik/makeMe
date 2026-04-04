package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"makeMe/internal/config"
	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
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

	assessRepo := postgres.NewAssessmentRepository(queries)
	assessSvc := assessment.NewService(assessRepo, userSvc)

	tgBot, err := telegram.New(cfg.TelegramToken, userSvc, groupSvc, assessSvc)
	if err != nil {
		return err
	}

	if err := tgBot.SetWebhook(ctx, cfg.WebhookURL); err != nil {
		return err
	}
	go tgBot.StartWebhook(ctx)

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
