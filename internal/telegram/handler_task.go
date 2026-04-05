package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/task"
)

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

func (b *Bot) handleTasks(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
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
			Text:   "Сначала пройди тест на определение уровня: /assess",
		})
		return
	}

	// Get user's groups
	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil || len(groups) == 0 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "У тебя нет групп. Создай группу (/newgroup) или вступи в существующую (/join).",
		})
		return
	}

	// Generate/get tasks for first group (MVP: single group)
	groupID := groups[0].ID
	ds, err := b.taskSvc.GenerateDaily(ctx, u.ID, groupID, u.Level)
	if err != nil {
		slog.Error("failed to generate daily set", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось загрузить задания. Попробуй позже.",
		})
		return
	}

	nextTask, err := b.taskSvc.GetNextUnanswered(ctx, ds.ID, u.ID)
	if err != nil {
		slog.Error("failed to get next task", "user_id", u.ID, "error", err)
		return
	}

	if nextTask == nil {
		answered, total, _ := b.taskSvc.GetProgress(ctx, ds.ID, u.ID)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("Все задания на сегодня выполнены! (%d/%d)\nВозвращайся завтра за новыми.", answered, total),
		})
		return
	}

	answered, total, _ := b.taskSvc.GetProgress(ctx, ds.ID, u.ID)
	sendTaskQuestion(ctx, tgBot, update.Message.Chat.ID, nextTask, answered+1, total)
}

func (b *Bot) handleTaskCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	// Parse: "task:123:2" → taskID=123, optionIdx=2
	data := update.CallbackQuery.Data
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 {
		return
	}

	taskID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}

	optionIdx, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	chatID := update.CallbackQuery.Message.Message.Chat.ID

	// Get the task to find the selected answer text
	t, err := b.taskSvc.GetTaskByID(ctx, taskID)
	if err != nil {
		return
	}

	response := ""
	if optionIdx >= 0 && optionIdx < len(t.Options) {
		response = t.Options[optionIdx]
	}

	result, err := b.taskSvc.SubmitAnswer(ctx, taskID, u.ID, response)
	if err != nil {
		if err == task.ErrAlreadyAnswered {
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Ты уже ответил на это задание.",
			})
			return
		}
		slog.Error("failed to submit answer", "user_id", u.ID, "task_id", taskID, "error", err)
		return
	}

	// Feedback
	var feedback string
	if result.Correct {
		feedback = "✅ Правильно!"
	} else {
		feedback = fmt.Sprintf("❌ Неправильно. Правильный ответ: %s", t.Answer)
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   feedback,
	})

	// Show next task or summary
	nextTask, _ := b.taskSvc.GetNextUnanswered(ctx, t.DailySetID, u.ID)
	if nextTask == nil {
		answered, total, _ := b.taskSvc.GetProgress(ctx, t.DailySetID, u.ID)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Все задания выполнены! (%d/%d)\nВозвращайся завтра!", answered, total),
		})
		return
	}

	answered, total, _ := b.taskSvc.GetProgress(ctx, t.DailySetID, u.ID)
	sendTaskQuestion(ctx, tgBot, chatID, nextTask, answered+1, total)
}

func sendTaskQuestion(ctx context.Context, tgBot *bot.Bot, chatID int64, t *task.Task, current, total int) {
	typeLabel := "📖 Словарь"
	if t.Type == task.TypeGrammar {
		typeLabel = "📝 Грамматика"
	}

	buttons := make([][]models.InlineKeyboardButton, len(t.Options))
	for i, opt := range t.Options {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: opt, CallbackData: fmt.Sprintf("task:%d:%d", t.ID, i)},
		}
	}

	// For vocab tasks, add a Listen button if we have a word_id
	if t.Type == task.TypeVocab && t.Metadata != nil {
		if wordID := toInt64(t.Metadata["word_id"]); wordID > 0 {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{Text: "🔊 Послушать", CallbackData: fmt.Sprintf("word_audio:%d", wordID)},
			})
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("%s — Задание %d/%d\n\n%s", typeLabel, current, total, t.Prompt),
		ParseMode: models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func (b *Bot) handleWordAudioCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	// Parse: "word_audio:123"
	parts := strings.SplitN(update.CallbackQuery.Data, ":", 2)
	if len(parts) != 2 {
		return
	}
	wordID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID

	word, err := b.srsRepo.GetWordByID(ctx, wordID)
	if err != nil {
		slog.Error("failed to get word for audio", "word_id", wordID, "error", err)
		return
	}

	// Lazily fetch from Free Dictionary API if audio not cached yet
	if word.AudioURL == "" && b.dictClient != nil {
		result, err := b.dictClient.Lookup(ctx, word.English)
		if err != nil {
			slog.Warn("dictionary lookup failed", "word", word.English, "error", err)
		} else if result.AudioURL != "" || result.Transcription != "" {
			_ = b.srsRepo.UpdateWordAudio(ctx, word.ID, result.Transcription, result.AudioURL)
			word.AudioURL = result.AudioURL
			word.Transcription = result.Transcription
		}
	}

	if word.AudioURL == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Произношение для «%s» не найдено.", word.English),
		})
		return
	}

	tgBot.SendAudio(ctx, &bot.SendAudioParams{
		ChatID:  chatID,
		Audio:   &models.InputFileString{Data: word.AudioURL},
		Caption: fmt.Sprintf("%s — %s", word.English, word.Russian),
	})
}
