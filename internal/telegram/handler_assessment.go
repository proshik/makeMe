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
	"makeMe/internal/domain/user"
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

	data := update.CallbackQuery.Data
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return
	}

	var optionIdx int
	if parts[1] == "idk" {
		optionIdx = assessment.SkipOption
	} else {
		var err error
		optionIdx, err = strconv.Atoi(parts[1])
		if err != nil {
			return
		}
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	chatID := update.CallbackQuery.Message.Message.Chat.ID

	session, nextQuestion, answers, err := b.assessSvc.SubmitAnswer(ctx, u.ID, optionIdx)
	if err != nil {
		slog.Error("failed to submit assessment answer", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "😔 Произошла ошибка. Попробуй /assess ещё раз.",
		})
		return
	}

	if nextQuestion == nil {
		// Assessment complete — send summary then final message
		if len(answers) > 0 {
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      formatSummary(answers),
				ParseMode: models.ParseModeMarkdown,
			})
		}

		stats := buildLevelStats(answers)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			ParseMode: models.ParseModeMarkdown,
			Text: fmt.Sprintf(
				"🎉 Тест завершён\\! Твой уровень: *%s* 🏅\n\n"+
					"%s\n\n"+
					"Теперь создай группу с друзьями и начните соревноваться:\n"+
					"👥 /newgroup Название — создать группу\n"+
					"🔗 /join КОД — вступить в существующую\n\n"+
					"Первые задания ждут тебя: /tasks",
				session.DeterminedLevel,
				formatLevelJustification(stats),
			),
		})
		return
	}

	count, _ := b.assessSvc.AnswerCount(ctx, session.ID)
	sendQuestion(ctx, tgBot, chatID, nextQuestion, count+1)
}

func sendQuestion(ctx context.Context, tgBot *bot.Bot, chatID int64, q *assessment.Question, questionNum int) {
	// Option buttons — one per row
	buttons := make([][]models.InlineKeyboardButton, len(q.Options))
	for i, opt := range q.Options {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: opt, CallbackData: fmt.Sprintf("assess:%d", i)},
		}
	}
	// "I don't know" button in a separate row at the bottom
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "🤷 Не знаю", CallbackData: "assess:idk"},
	})

	const vocabPhase = 15
	var phaseLabel string
	var localNum, phaseTotal int
	if questionNum <= vocabPhase {
		phaseLabel = "📚 Словарный запас"
		localNum = questionNum
		phaseTotal = vocabPhase
	} else {
		phaseLabel = "✍️ Грамматика"
		localNum = questionNum - vocabPhase
		phaseTotal = 15
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("%s — Вопрос %d/%d:\n\n%s", phaseLabel, localNum, phaseTotal, q.Text),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

// formatSummary builds the detailed question-by-question breakdown.
func formatSummary(answers []*assessment.Answer) string {
	var sb strings.Builder
	sb.WriteString("📊 *Разбор теста*\n\n")

	for i, a := range answers {
		var icon string
		switch {
		case a.Skipped:
			icon = "🤷"
		case a.Correct:
			icon = "✅"
		default:
			icon = "❌"
		}

		sb.WriteString(fmt.Sprintf("%d\\. \\[%s\\] %s %s\n",
			i+1, escapeMD(string(a.QuestionLevel)), escapeMD(a.QuestionText), icon))

		correctAnswer := ""
		if a.CorrectOption >= 0 && a.CorrectOption < len(a.Options) {
			correctAnswer = a.Options[a.CorrectOption]
		}

		if a.Skipped {
			sb.WriteString(fmt.Sprintf("   _Правильный ответ: %s_\n", escapeMD(correctAnswer)))
		} else if !a.Correct && a.UserOption >= 0 && a.UserOption < len(a.Options) {
			userAnswer := a.Options[a.UserOption]
			sb.WriteString(fmt.Sprintf("   _Ваш ответ: %s_ → _Правильно: %s_\n",
				escapeMD(userAnswer), escapeMD(correctAnswer)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

type levelStat struct {
	Correct int
	Total   int
}

func buildLevelStats(answers []*assessment.Answer) map[user.Level]levelStat {
	stats := make(map[user.Level]levelStat)
	for _, a := range answers {
		if a.Skipped {
			continue
		}
		s := stats[a.QuestionLevel]
		s.Total++
		if a.Correct {
			s.Correct++
		}
		stats[a.QuestionLevel] = s
	}
	return stats
}

func formatLevelJustification(stats map[user.Level]levelStat) string {
	allLevels := []user.Level{user.A1, user.A2, user.B1, user.B2, user.C1, user.C2}
	var sb strings.Builder
	for _, lv := range allLevels {
		s, ok := stats[lv]
		if !ok {
			continue
		}
		pct := 0
		if s.Total > 0 {
			pct = s.Correct * 100 / s.Total
		}
		icon := "❌"
		if s.Total > 0 && float64(s.Correct)/float64(s.Total) >= 0.5 {
			icon = "✅"
		}
		sb.WriteString(fmt.Sprintf("%s %s: %d/%d \\(%d%%\\)\n",
			icon, escapeMD(string(lv)), s.Correct, s.Total, pct))
	}
	return sb.String()
}

// escapeMD escapes MarkdownV2 special characters.
func escapeMD(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}
