package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/dictionary"
	"makeMe/internal/domain/assessment"
	"makeMe/internal/domain/group"
	"makeMe/internal/domain/scoring"
	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
	"makeMe/internal/domain/words"
)

type Bot struct {
	bot         *bot.Bot
	userSvc     *user.Service
	groupSvc    *group.Service
	assessSvc   *assessment.Service
	taskSvc     *task.Service
	scoringSvc  *scoring.Service
	srsRepo     srs.Repository
	dictClient  *dictionary.Client
	grammarRepo task.GrammarRepository
	wordsSvc    *words.Service
	pendingRepo words.PendingRepository
}

func New(token string, userSvc *user.Service, groupSvc *group.Service, assessSvc *assessment.Service, taskSvc *task.Service, scoringSvc *scoring.Service, srsRepo srs.Repository, dictClient *dictionary.Client, grammarRepo task.GrammarRepository, wordsSvc *words.Service, pendingRepo words.PendingRepository, extraOpts ...bot.Option) (*Bot, error) {
	b := &Bot{
		userSvc:     userSvc,
		groupSvc:    groupSvc,
		assessSvc:   assessSvc,
		taskSvc:     taskSvc,
		scoringSvc:  scoringSvc,
		srsRepo:     srsRepo,
		dictClient:  dictClient,
		grammarRepo: grammarRepo,
		wordsSvc:    wordsSvc,
		pendingRepo: pendingRepo,
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
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/learn", bot.MatchTypeExact, b.handleLearn)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "learn:", bot.MatchTypePrefix, b.handleLearnCallback)
	tgBot.RegisterHandler(bot.HandlerTypeMessageText, "/words", bot.MatchTypeExact, b.handleWords)
	tgBot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "words:", bot.MatchTypePrefix, b.handleWordsCallback)

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

// defaultHandler dispatches plain-text messages that don't match a registered
// command. If the user has a pending typing prompt, the reply is routed to
// the appropriate typing handler. Otherwise we show a hint.
func (b *Bot) defaultHandler(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	// Unknown command (matches "/anything" not registered above) — preserve original behavior.
	if strings.HasPrefix(text, "/") {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Неизвестная команда. Отправь /start чтобы начать.",
		})
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	if b.pendingRepo == nil {
		// Pending repo not wired (e.g. minimal-deps tests) — fall back to hint.
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Используй команды: /tasks /words /learn /help",
		})
		return
	}

	pending, err := b.pendingRepo.Get(ctx, u.ID)
	if err != nil {
		slog.Error("failed to get pending typing", "user_id", u.ID, "error", err)
		return
	}
	if pending == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Просто текст? Используй команды: /tasks /words /learn /help",
		})
		return
	}
	if pending.ExpiresAt.Before(time.Now()) {
		_ = b.pendingRepo.Delete(ctx, u.ID)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "⏰ Время на ответ истекло. Запусти /words или /tasks снова.",
		})
		return
	}

	// Single-use: consume now, regardless of dispatch outcome.
	_ = b.pendingRepo.Delete(ctx, u.ID)
	b.handleTypingReply(ctx, tgBot, update, pending)
}
