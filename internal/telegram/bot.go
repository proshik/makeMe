package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/group"
	"makeMe/internal/domain/user"
)

type Bot struct {
	bot      *bot.Bot
	userSvc  *user.Service
	groupSvc *group.Service
}

func New(token string, userSvc *user.Service, groupSvc *group.Service, extraOpts ...bot.Option) (*Bot, error) {
	b := &Bot{
		userSvc:  userSvc,
		groupSvc: groupSvc,
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
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/newgroup ", bot.MatchTypePrefix, b.handleNewGroup)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/join ", bot.MatchTypePrefix, b.handleJoin)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/mygroups", bot.MatchTypeExact, b.handleMyGroups)

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

func (b *Bot) defaultHandler(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Неизвестная команда. Отправь /start чтобы начать.",
	})
}
