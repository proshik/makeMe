package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/words"
)

// handleTypingReply is dispatched from defaultHandler when the user sends a
// plain-text message and a pending typing prompt exists for that user. It
// routes to either /words or /tasks typing handlers depending on pending.Source.
func (b *Bot) handleTypingReply(ctx context.Context, tgBot *bot.Bot, update *models.Update, pending *words.PendingTyping) {
	if update.Message == nil {
		return
	}
	u := UserFromContext(ctx)
	if u == nil {
		return
	}
	chatID := update.Message.Chat.ID
	raw := update.Message.Text

	switch pending.Source {
	case "words":
		b.handleWordsTypingReply(ctx, tgBot, chatID, u.ID, raw, pending.Payload)
	case "tasks":
		b.handleTasksTypingReply(ctx, tgBot, chatID, u.ID, raw, pending.Payload)
	default:
		slog.Warn("unknown pending typing source", "source", pending.Source, "user_id", u.ID)
	}
}
