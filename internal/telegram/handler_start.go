package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) handleStart(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
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
				"Для начала нужно определить твой уровень — скоро будет доступен тест.\n\n"+
				"Доступные команды:\n"+
				"/newgroup Название — создать группу\n"+
				"/join КОД — вступить в группу\n"+
				"/mygroups — мои группы",
			u.FirstName,
		)
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	})
}
