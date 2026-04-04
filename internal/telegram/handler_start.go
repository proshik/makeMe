package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) handleStart(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	telegramID := msg.From.ID
	username := msg.From.Username
	firstName := msg.From.FirstName

	u, err := b.userSvc.Register(ctx, telegramID, username, firstName)
	if err != nil {
		slog.Error("failed to register user", "telegram_id", telegramID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "Произошла ошибка. Попробуйте позже.",
		})
		return
	}

	var text string
	if u.Onboarded {
		text = fmt.Sprintf("С возвращением, %s! Твой уровень: %s.", u.FirstName, u.Level)
	} else {
		text = fmt.Sprintf(
			"Привет, %s! Добро пожаловать в makeMe!\n\n"+
				"Здесь ты будешь учить английский, соревнуясь с друзьями.\n"+
				"Для начала нужно определить твой уровень — скоро будет доступен тест.",
			u.FirstName,
		)
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   text,
	})
}
