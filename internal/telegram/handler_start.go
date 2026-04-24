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

	u := UserFromContext(ctx)
	if u == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Произошла ошибка. Попробуйте позже.",
		})
		return
	}

	slog.Debug("cmd received", "cmd", "/start", "user_id", u.ID)
	slog.Info("user start", "user_id", u.ID, "onboarded", u.Onboarded, "level", u.Level)

	var text string
	if u.Onboarded {
		text = fmt.Sprintf(
			"👋 С возвращением, %s! Продолжаем покорять английский 💪\n\n"+
				"Твой уровень: %s\n\n"+
				"👉 /tasks — задания на сегодня\n"+
				"👉 /leaderboard — рейтинг группы\n\n"+
				"👉 /help — как всё устроено",
			u.FirstName, u.Level,
		)
	} else {
		text = fmt.Sprintf(
			"🎉 Привет, %s! Добро пожаловать в makeMe — игровой тренажёр английского!\n\n"+
				helpText(),
			u.FirstName,
		)
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	})
}

func (b *Bot) handleHelp(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "📖 makeMe — как всё устроено\n\n" + helpText(),
	})
}

// helpText returns the shared learning process description used in /start and /help.
func helpText() string {
	return "Как это работает:\n\n" +
		"1️⃣ Определи свой уровень — пройди короткий тест (10 вопросов):\n" +
		"   /assess\n\n" +
		"2️⃣ Создай группу с друзьями или вступи в существующую:\n" +
		"   /newgroup Название — создать группу\n" +
		"   /join КОД — вступить по инвайт-коду\n\n" +
		"3️⃣ Выполняй задания каждый день — новый набор каждые 24 часа:\n" +
		"   /tasks — получить задания на сегодня\n\n" +
		"   Задания бывают двух типов:\n" +
		"   📖 Словарь — выбери правильный перевод слова\n" +
		"   📝 Грамматика — вставь пропущенное слово или форму\n\n" +
		"4️⃣ Зарабатывай баллы — за каждый правильный ответ начисляются очки:\n" +
		"   10 очков × коэффициент уровня задания:\n" +
		"   A1 × 0.6 = 6 pts   A2 × 0.8 = 8 pts   B1 × 1.0 = 10 pts\n" +
		"   B2 × 1.15 = 11.5   C1 × 1.3 = 13 pts   C2 × 1.5 = 15 pts\n" +
		"   Игроки разных уровней соревнуются честно 🏅\n\n" +
		"5️⃣ Следи за рейтингом в своей группе:\n" +
		"   /leaderboard — таблица лидеров за день и за месяц\n\n" +
		"──────────────────\n" +
		"📋 Все команды:\n\n" +
		"/assess — пройти тест на определение уровня\n" +
		"/tasks — задания на сегодня\n" +
		"/learn — изучить грамматику по темам\n" +
		"/newgroup Название — создать группу\n" +
		"/join КОД — вступить в группу\n" +
		"/mygroups — мои группы и инвайт-коды\n" +
		"/leaderboard — рейтинг группы\n" +
		"/profile — моя статистика (уровень, баллы, серия, слова)\n\n" +
		"/help — эта справка"
}
