package telegram

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/user"
)

type ctxKey string

const userCtxKey ctxKey = "user"

// UserFromContext returns the user stored in the context by the user-resolve middleware.
func UserFromContext(ctx context.Context) *user.User {
	u, _ := ctx.Value(userCtxKey).(*user.User)
	return u
}

// userResolveMiddleware looks up (or creates) the user by telegram_id
// and stores it in the context for downstream handlers.
func (b *Bot) userResolveMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
		telegramID, username, firstName := extractSender(update)
		if telegramID == 0 {
			next(ctx, tgBot, update)
			return
		}

		u, err := b.userSvc.Register(ctx, telegramID, username, firstName)
		if err != nil {
			slog.Error("failed to resolve user", "telegram_id", telegramID, "error", err)
			next(ctx, tgBot, update)
			return
		}

		ctx = context.WithValue(ctx, userCtxKey, u)

		// If the user just issued a /command, clear any stale pending typing
		// prompt so their next text reply isn't accidentally consumed by an
		// earlier session.
		if b.pendingRepo != nil && update.Message != nil && strings.HasPrefix(update.Message.Text, "/") {
			_ = b.pendingRepo.Delete(ctx, u.ID)
		}

		next(ctx, tgBot, update)
	}
}

func extractSender(update *models.Update) (int64, string, string) {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.ID, update.Message.From.Username, update.Message.From.FirstName
	}
	if update.CallbackQuery != nil && update.CallbackQuery.From.ID != 0 {
		return update.CallbackQuery.From.ID, update.CallbackQuery.From.Username, update.CallbackQuery.From.FirstName
	}
	return 0, "", ""
}
