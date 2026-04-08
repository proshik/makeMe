package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/assessment"
)

func (b *Bot) handleAssess(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	if u.Onboarded {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("🎓 Ты уже прошёл оценку! Твой уровень: %s — отличный результат!", u.Level),
		})
		return
	}

	session, question, err := b.assessSvc.Start(ctx, u.ID)
	if err != nil {
		slog.Error("failed to start assessment", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "😔 Не удалось начать тест. Попробуй позже.",
		})
		return
	}

	count, _ := b.assessSvc.AnswerCount(ctx, session.ID)
	sendQuestion(ctx, tgBot, update.Message.Chat.ID, question, count+1)
}

func (b *Bot) handleAssessCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	// Parse callback data: "assess:0", "assess:1", etc.
	data := update.CallbackQuery.Data
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return
	}

	optionIdx, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	// Answer the callback to remove loading indicator
	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	chatID := update.CallbackQuery.Message.Message.Chat.ID

	session, nextQuestion, err := b.assessSvc.SubmitAnswer(ctx, u.ID, optionIdx)
	if err != nil {
		slog.Error("failed to submit assessment answer", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "😔 Произошла ошибка. Попробуй /assess ещё раз.",
		})
		return
	}

	if nextQuestion == nil {
		// Assessment complete
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: fmt.Sprintf(
				"🎉 Тест завершён! Твой уровень: *%s* 🏅\n\n"+
					"Теперь создай группу с друзьями и начните соревноваться:\n"+
					"👥 /newgroup Название — создать группу\n"+
					"🔗 /join КОД — вступить в существующую\n\n"+
					"Первые задания ждут тебя: /tasks",
				session.DeterminedLevel,
			),
		})
		return
	}

	count, _ := b.assessSvc.AnswerCount(ctx, session.ID)
	sendQuestion(ctx, tgBot, chatID, nextQuestion, count+1)
}

func sendQuestion(ctx context.Context, tgBot *bot.Bot, chatID int64, q *assessment.Question, questionNum int) {
	buttons := make([][]models.InlineKeyboardButton, len(q.Options))
	for i, opt := range q.Options {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: opt, CallbackData: fmt.Sprintf("assess:%d", i)},
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("🧠 Вопрос %d/10:\n\n%s", questionNum, q.Text),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}
