package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/dictionary"
	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
)

type Bot struct {
	bot        *bot.Bot
	userSvc    *user.Service
	groupSvc   *group.Service
	assessSvc  *assessment.Service
	taskSvc    *task.Service
	scoringSvc *scoring.Service
	srsRepo    srs.Repository
	dictClient *dictionary.Client
}

func New(token string, userSvc *user.Service, groupSvc *group.Service, assessSvc *assessment.Service, taskSvc *task.Service, scoringSvc *scoring.Service, srsRepo srs.Repository, dictClient *dictionary.Client, extraOpts ...bot.Option) (*Bot, error) {
	b := &Bot{
		userSvc:    userSvc,
		groupSvc:   groupSvc,
		assessSvc:  assessSvc,
		taskSvc:    taskSvc,
		scoringSvc: scoringSvc,
		srsRepo:    srsRepo,
		dictClient: dictClient,
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.defaultHandler),
		bot.WithMiddlewares(b.userResolveMiddleware),
	}
	opts = append(opts, extraOpts...)

	tgBot, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, b.handleHelp)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/assess", bot.MatchTypeExact, b.handleAssess)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/newgroup", bot.MatchTypePrefix, b.handleNewGroup)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/join", bot.MatchTypePrefix, b.handleJoin)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/mygroups", bot.MatchTypeExact, b.handleMyGroups)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/tasks", bot.MatchTypeExact, b.handleTasks)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "assess:", bot.MatchTypePrefix, b.handleAssessCallback)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "task:", bot.MatchTypePrefix, b.handleTaskCallback)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "word_audio:", bot.MatchTypePrefix, b.handleWordAudioCallback)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/leaderboard", bot.MatchTypeExact, b.handleLeaderboard)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "lb:", bot.MatchTypePrefix, b.handleLeaderboardCallback)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/profile", bot.MatchTypeExact, b.handleProfile)

	b.bot = tgBot
	return b, nil
}

// WebhookHandler returns an http.HandlerFunc for the chi router.
func (b *Bot) WebhookHandler() http.HandlerFunc {
	return b.bot.WebhookHandler()
}

// SetWebhook registers the webhook URL with Telegram.
func (b *Bot) SetWebhook(ctx context.Context, webhookURL string) error {
	_, err := b.bot.SetWebhook(ctx, &bot.SetWebhookParams{
		URL: webhookURL,
	})
	if err != nil {
		return fmt.Errorf("set webhook: %w", err)
	}
	slog.Info("telegram webhook set", "url", webhookURL)
	return nil
}

// StartWebhook starts processing updates received via webhook.
func (b *Bot) StartWebhook(ctx context.Context) {
	b.bot.StartWebhook(ctx)
}

// SendMessage sends a proactive message to a chat (e.g. from the scheduler).
func (b *Bot) SendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func (b *Bot) defaultHandler(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Неизвестная команда. Отправь /start чтобы начать.",
	})
}
