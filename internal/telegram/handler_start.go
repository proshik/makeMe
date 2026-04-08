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
		text = fmt.Sprintf("👋 С возвращением, %s! Продолжаем покорять английский 💪\n\nТвой уровень: %s", u.FirstName, u.Level)
	} else {
		text = fmt.Sprintf(
			"🎉 Привет, %s! Добро пожаловать в makeMe!\n\n"+
				"Здесь ты учишь английский в игровой форме и соревнуешься с друзьями 🏆\n\n"+
				"Для начала пройди короткий тест — он определит твой уровень:\n"+
				"👉 /assess\n\n"+
				"📋 Доступные команды:\n"+
				"/assess — пройти тест на уровень\n"+
				"/newgroup Название — создать группу\n"+
				"/join КОД — вступить в группу\n"+
				"/mygroups — мои группы\n"+
				"/tasks — задания на сегодня\n"+
				"/profile — моя статистика",
			u.FirstName,
		)
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	})
}
