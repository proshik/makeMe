package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/user"
)

const learnPracticeLimit = 5

// nextLevel returns the next CEFR level above l, or ("", false) for C2.
func nextLevel(l user.Level) (user.Level, bool) {
	levels := []user.Level{user.A1, user.A2, user.B1, user.B2, user.C1, user.C2}
	for i, lv := range levels {
		if lv == l && i+1 < len(levels) {
			return levels[i+1], true
		}
	}
	return "", false
}

func (b *Bot) handleLearn(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	u := UserFromContext(ctx)
	if u == nil {
		return
	}
	if !u.Onboarded {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "🎓 Сначала пройди тест на определение уровня: /assess",
		})
		return
	}
	b.sendLearnMenu(ctx, tgBot, update.Message.Chat.ID, u.Level)
}

func (b *Bot) handleLearnCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	data := update.CallbackQuery.Data

	switch {
	case data == "learn:menu":
		b.sendLearnMenu(ctx, tgBot, chatID, u.Level)

	case strings.HasPrefix(data, "learn:cat:"):
		b.handleLearnCat(ctx, tgBot, chatID, data)

	case strings.HasPrefix(data, "learn:practice:"):
		b.handleLearnPractice(ctx, tgBot, chatID, data, 0)

	case strings.HasPrefix(data, "learn:qa:"):
		b.handleLearnAnswer(ctx, tgBot, chatID, data)
	}
}

// sendLearnMenu fetches categories for current and next level and sends the menu.
func (b *Bot) sendLearnMenu(ctx context.Context, tgBot *bot.Bot, chatID int64, level user.Level) {
	currentCats, err := b.grammarRepo.GetCategoriesByLevel(ctx, level)
	if err != nil {
		slog.Error("failed to get grammar categories", "level", level, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "😔 Не удалось загрузить темы. Попробуй позже.",
		})
		return
	}

	next, hasNext := nextLevel(level)
	var nextCats []task.GrammarCategory
	if hasNext {
		nextCats, err = b.grammarRepo.GetCategoriesByLevel(ctx, next)
		if err != nil {
			slog.Error("failed to get next level grammar categories", "level", next, "error", err)
		}
	}

	text := fmt.Sprintf("📚 <b>Выбери тему для изучения:</b>\n\n📗 Твой уровень (%s):", level)
	if hasNext {
		text += fmt.Sprintf("\n\nПосле освоения — переходи к 📘 <b>%s</b>", next)
	}

	keyboard := buildLearnMenuKeyboard(currentCats, level, nextCats, next)

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	})
}

// buildLearnMenuKeyboard builds two buttons per row for categories; current level first, then next level with a separator row.
func buildLearnMenuKeyboard(currentCats []task.GrammarCategory, currentLevel user.Level, nextCats []task.GrammarCategory, nextLevel user.Level) [][]models.InlineKeyboardButton {
	var rows [][]models.InlineKeyboardButton

	rows = append(rows, catButtons(currentCats, currentLevel)...)

	if len(nextCats) > 0 {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("── %s (следующий уровень) ──", nextLevel), CallbackData: "learn:noop"},
		})
		rows = append(rows, catButtons(nextCats, nextLevel)...)
	}

	return rows
}

// catButtons groups category buttons two per row.
func catButtons(cats []task.GrammarCategory, level user.Level) [][]models.InlineKeyboardButton {
	var rows [][]models.InlineKeyboardButton
	for i := 0; i < len(cats); i += 2 {
		btn := models.InlineKeyboardButton{
			Text:         cats[i].Name,
			CallbackData: fmt.Sprintf("learn:cat:%s:%d", level, i),
		}
		if i+1 < len(cats) {
			rows = append(rows, []models.InlineKeyboardButton{
				btn,
				{
					Text:         cats[i+1].Name,
					CallbackData: fmt.Sprintf("learn:cat:%s:%d", level, i+1),
				},
			})
		} else {
			rows = append(rows, []models.InlineKeyboardButton{btn})
		}
	}
	return rows
}

// handleLearnCat parses "learn:cat:{level}:{idx}" and shows the theory screen.
func (b *Bot) handleLearnCat(ctx context.Context, tgBot *bot.Bot, chatID int64, data string) {
	level, idx, ok := parseLearnCat(data)
	if !ok {
		return
	}

	cats, err := b.grammarRepo.GetCategoriesByLevel(ctx, level)
	if err != nil || idx >= len(cats) {
		slog.Error("failed to get categories for learn cat", "error", err)
		return
	}

	cat := cats[idx]

	// Count available practice questions
	questions, err := b.grammarRepo.GetQuestionsByCategory(ctx, cat.Name, learnPracticeLimit)
	if err != nil {
		slog.Error("failed to count questions for category", "category", cat.Name, "error", err)
	}
	total := len(questions)

	var theoryText string
	if cat.Explanation != "" {
		theoryText = fmt.Sprintf("📗 <b>%s</b>\n\n%s", html.EscapeString(cat.Name), formatExplanationHTML(cat.Explanation))
	} else {
		theoryText = fmt.Sprintf("📗 <b>%s</b>\n\nОписание для этой темы пока не добавлено.", html.EscapeString(cat.Name))
	}

	catCallback := fmt.Sprintf("learn:cat:%s:%d", level, idx)
	var keyboard [][]models.InlineKeyboardButton
	if total > 0 {
		keyboard = [][]models.InlineKeyboardButton{
			{
				{Text: fmt.Sprintf("📝 Начать практику (%d вопросов) →", total),
					CallbackData: fmt.Sprintf("learn:practice:%s:%d:%d", level, idx, total)},
			},
			{
				{Text: "← Назад к темам", CallbackData: "learn:menu"},
			},
		}
	} else {
		keyboard = [][]models.InlineKeyboardButton{
			{{Text: "← Назад к темам", CallbackData: "learn:menu"}},
		}
	}
	_ = catCallback

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      theoryText,
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	})
}

// handleLearnPractice shows practice question at position qIdx.
// Called for "learn:practice:{level}:{catIdx}:{total}" (first question).
func (b *Bot) handleLearnPractice(ctx context.Context, tgBot *bot.Bot, chatID int64, data string, qIdx int) {
	level, catIdx, total, ok := parseLearnPractice(data)
	if !ok {
		return
	}
	b.sendLearnQuestion(ctx, tgBot, chatID, level, catIdx, qIdx, total)
}

// handleLearnAnswer processes "learn:qa:{level}:{catIdx}:{qIdx}:{total}:{optIdx}".
func (b *Bot) handleLearnAnswer(ctx context.Context, tgBot *bot.Bot, chatID int64, data string) {
	level, catIdx, qIdx, total, optIdx, ok := parseLearnQA(data)
	if !ok {
		return
	}

	cats, err := b.grammarRepo.GetCategoriesByLevel(ctx, level)
	if err != nil || catIdx >= len(cats) {
		return
	}

	questions, err := b.grammarRepo.GetQuestionsByCategory(ctx, cats[catIdx].Name, learnPracticeLimit)
	if err != nil || qIdx >= len(questions) {
		return
	}

	q := questions[qIdx]
	correct := optIdx >= 0 && optIdx < len(q.Options) && q.Options[optIdx] == q.Answer

	var feedbackText string
	if correct {
		feedbackText = "✅ Правильно!"
	} else {
		correctAnswer := q.Answer
		feedbackText = fmt.Sprintf("❌ Неправильно. Правильный ответ: <b>%s</b>", html.EscapeString(correctAnswer))
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      feedbackText,
		ParseMode: models.ParseModeHTML,
	})

	nextIdx := qIdx + 1
	if nextIdx < total {
		b.sendLearnQuestion(ctx, tgBot, chatID, level, catIdx, nextIdx, total)
	} else {
		// Practice complete
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "✅ Практика завершена! Отличная работа 💪",
			ParseMode: models.ParseModeHTML,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: "← Назад к темам", CallbackData: "learn:menu"}},
				},
			},
		})
	}
}

// sendLearnQuestion fetches and displays question qIdx for the given category.
func (b *Bot) sendLearnQuestion(ctx context.Context, tgBot *bot.Bot, chatID int64, level user.Level, catIdx, qIdx, total int) {
	cats, err := b.grammarRepo.GetCategoriesByLevel(ctx, level)
	if err != nil || catIdx >= len(cats) {
		slog.Error("failed to get categories for learn question", "error", err)
		return
	}

	questions, err := b.grammarRepo.GetQuestionsByCategory(ctx, cats[catIdx].Name, learnPracticeLimit)
	if err != nil || qIdx >= len(questions) {
		slog.Error("failed to get questions for category", "category", cats[catIdx].Name, "error", err)
		return
	}

	q := questions[qIdx]

	buttons := make([][]models.InlineKeyboardButton, len(q.Options))
	for i, opt := range q.Options {
		buttons[i] = []models.InlineKeyboardButton{
			{
				Text:         opt,
				CallbackData: fmt.Sprintf("learn:qa:%s:%d:%d:%d:%d", level, catIdx, qIdx, total, i),
			},
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("📝 Вопрос %d/%d\nВыберите правильный вариант, чтобы заполнить пропуск:\n\n%s", qIdx+1, total, html.EscapeString(q.Prompt)),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

// --- Callback parsers ---

// parseLearnCat parses "learn:cat:{level}:{idx}".
func parseLearnCat(data string) (user.Level, int, bool) {
	// "learn:cat:B1:3" → split on ":" gives ["learn", "cat", "B1", "3"]
	parts := strings.SplitN(data, ":", 4)
	if len(parts) != 4 {
		return "", 0, false
	}
	idx, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, false
	}
	return user.Level(parts[2]), idx, true
}

// parseLearnPractice parses "learn:practice:{level}:{catIdx}:{total}".
func parseLearnPractice(data string) (user.Level, int, int, bool) {
	parts := strings.SplitN(data, ":", 5)
	if len(parts) != 5 {
		return "", 0, 0, false
	}
	catIdx, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, 0, false
	}
	total, err := strconv.Atoi(parts[4])
	if err != nil {
		return "", 0, 0, false
	}
	return user.Level(parts[2]), catIdx, total, true
}

// parseLearnQA parses "learn:qa:{level}:{catIdx}:{qIdx}:{total}:{optIdx}".
func parseLearnQA(data string) (user.Level, int, int, int, int, bool) {
	parts := strings.SplitN(data, ":", 7)
	if len(parts) != 7 {
		return "", 0, 0, 0, 0, false
	}
	catIdx, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	qIdx, err := strconv.Atoi(parts[4])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	total, err := strconv.Atoi(parts[5])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	optIdx, err := strconv.Atoi(parts[6])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	return user.Level(parts[2]), catIdx, qIdx, total, optIdx, true
}
