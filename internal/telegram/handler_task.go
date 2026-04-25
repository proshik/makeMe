package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/task"
	"makeMe/internal/domain/words"
)

// pluralRounds returns the Russian plural form for "раунд" matching n.
func pluralRounds(n int) string {
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 14 {
		return "раундов"
	}
	switch n % 10 {
	case 1:
		return "раунд"
	case 2, 3, 4:
		return "раунда"
	default:
		return "раундов"
	}
}

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

	slog.Debug("cmd received", "cmd", "/tasks", "user_id", u.ID)

	if !u.Onboarded {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "🎓 Сначала пройди тест на определение уровня: /assess",
		})
		return
	}

	// Get user's groups — auto-create a solo group if none exist
	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil {
		slog.Error("failed to list groups", "user_id", u.ID, "error", err)
		return
	}
	if len(groups) == 0 {
		g, err := b.groupSvc.Create(ctx, "Соло", u.ID)
		if err != nil {
			slog.Error("failed to create solo group", "user_id", u.ID, "error", err)
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Не удалось создать группу. Попробуй позже.",
			})
			return
		}
		groups = append(groups, *g)
	}

	// Generate/get tasks for first group (MVP: single group)
	groupID := groups[0].ID
	ds, err := b.taskSvc.GenerateDaily(ctx, u.ID, groupID, u.Level)
	if err != nil {
		if errors.Is(err, task.ErrDailyLimitReached) {
			slog.Info("daily limit reached", "user_id", u.ID, "max_rounds", b.taskSvc.MaxRounds())
			max := b.taskSvc.MaxRounds()
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: fmt.Sprintf(
					"🎯 Сегодняшний лимит исчерпан.\n\nКаждому пользователю выдаётся по %d %s за один день. Возвращайся завтра за новыми заданиями.",
					max, pluralRounds(max),
				),
			})
			return
		}
		slog.Error("failed to generate daily set", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось загрузить задания. Попробуй позже.",
		})
		return
	}

	slog.Info("task round started", "user_id", u.ID, "group_id", groupID, "round", ds.Round+1, "max_rounds", b.taskSvc.MaxRounds())

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("🎯 Раунд %d из %d на сегодня", ds.Round+1, b.taskSvc.MaxRounds()),
	})

	nextTask, err := b.taskSvc.GetNextUnanswered(ctx, ds.ID, u.ID)
	if err != nil {
		slog.Error("failed to get next task", "user_id", u.ID, "error", err)
		return
	}

	if nextTask == nil {
		_, total, _ := b.taskSvc.GetProgress(ctx, ds.ID, u.ID)
		correct, _ := b.taskSvc.GetCorrectCount(ctx, ds.ID, u.ID)
		var doneText string
		if correct == total {
			doneText = fmt.Sprintf("🎉 Все задания на сегодня выполнены!\n\n✅ Правильных ответов: %d из %d\n\nОтличная работа! Возвращайся завтра за новыми 💪", correct, total)
		} else {
			doneText = fmt.Sprintf("📝 Задания на сегодня выполнены.\n\n✅ Правильных ответов: %d из %d\n\nВозвращайся завтра — новые задания уже ждут 💪", correct, total)
		}
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   doneText,
		})
		return
	}

	answered, total, _ := b.taskSvc.GetProgress(ctx, ds.ID, u.ID)
	slog.Debug("task shown", "user_id", u.ID, "task_id", nextTask.ID, "type", nextTask.Type, "level", nextTask.Level, "position", answered+1, "total", total)
	b.sendTaskQuestion(ctx, tgBot, update.Message.Chat.ID, u.ID, nextTask, answered+1, total)
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

	isSkip := parts[2] == "idk"
	slog.Debug("task callback", "user_id", u.ID, "task_id", parts[1], "is_skip", isSkip)

	var optionIdx int
	if !isSkip {
		optionIdx, err = strconv.Atoi(parts[2])
		if err != nil {
			return
		}
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

	var result *task.TaskResult
	if isSkip {
		result, err = b.taskSvc.SubmitSkip(ctx, taskID, u.ID)
	} else {
		response := ""
		if optionIdx >= 0 && optionIdx < len(t.Options) {
			response = t.Options[optionIdx]
		}
		result, err = b.taskSvc.SubmitAnswer(ctx, taskID, u.ID, response)
	}
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

	slog.Info("task answered", "user_id", u.ID, "task_id", taskID, "type", t.Type, "level", t.Level, "correct", result.Correct, "skipped", result.Skipped, "score", result.Score)

	// Feedback
	var feedback string
	correctPhrases := []string{"✅ Правильно!", "✅ Отлично!", "✅ Молодец!", "✅ В точку!", "✅ Супер!"}
	switch {
	case result.Skipped:
		feedback = buildSkipFeedback(t)
	case result.Correct:
		feedback = correctPhrases[taskID%int64(len(correctPhrases))]
		if b.scoringSvc != nil {
			if err := b.scoringSvc.RecordScore(ctx, u.ID, t.GroupID, t.Level, result.Score); err != nil {
				slog.Error("failed to record score", "user_id", u.ID, "task_id", taskID, "error", err)
			}
		}
	default:
		feedback = buildIncorrectFeedback(t)
	}
	feedbackParams := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      feedback,
		ParseMode: models.ParseModeHTML,
	}
	// Always offer the audio button for vocab tasks: handleWordAudioCallback
	// lazily fetches pronunciation on first click and falls back to a
	// "not found" reply when the word has no audio.
	if t.Type == task.TypeVocab && t.Metadata != nil {
		if wordID := toInt64(t.Metadata["word_id"]); wordID > 0 {
			feedbackParams.ReplyMarkup = &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: "🔊 Послушать", CallbackData: fmt.Sprintf("word_audio:%d", wordID)}},
				},
			}
		}
	}
	tgBot.SendMessage(ctx, feedbackParams)

	// Show next task or summary
	nextTask, _ := b.taskSvc.GetNextUnanswered(ctx, t.DailySetID, u.ID)
	if nextTask == nil {
		_, total, _ := b.taskSvc.GetProgress(ctx, t.DailySetID, u.ID)
		correct, _ := b.taskSvc.GetCorrectCount(ctx, t.DailySetID, u.ID)
		slog.Info("round completed", "user_id", u.ID, "daily_set_id", t.DailySetID, "correct", correct, "total", total)
		var completionText string
		if correct == total {
			completionText = fmt.Sprintf("🏆 Все задания выполнены!\n\n✅ Правильных ответов: %d из %d\n\nТы огонь! Загляни в /leaderboard — посмотри, где ты в рейтинге 🔥", correct, total)
		} else {
			completionText = fmt.Sprintf("✅ Задания выполнены!\n\n✅ Правильных ответов: %d из %d\n\nЗагляни в /leaderboard — посмотри, где ты в рейтинге 🔥", correct, total)
		}
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   completionText,
		})
		return
	}

	answered, total, _ := b.taskSvc.GetProgress(ctx, t.DailySetID, u.ID)
	slog.Debug("task shown", "user_id", u.ID, "task_id", nextTask.ID, "type", nextTask.Type, "level", nextTask.Level, "position", answered+1, "total", total)
	b.sendTaskQuestion(ctx, tgBot, chatID, u.ID, nextTask, answered+1, total)
}

// vocabTaskLabel returns a display label for a vocab task based on its subtype.
func vocabTaskLabel(t *task.Task) string {
	if t.Metadata == nil {
		return "📖 Словарь"
	}
	switch t.Metadata["subtype"] {
	case task.VocabSubtypeRuEn:
		return "📖 Перевод слов"
	case task.VocabSubtypeFillBlank:
		return "📝 Заполни пропуск"
	case task.VocabSubtypeEnRu:
		return "📖 Перевод слов"
	case task.VocabSubtypeTypingEnRu, task.VocabSubtypeTypingRuEn:
		return "✍️ Введи перевод"
	default:
		return "📖 Словарь"
	}
}

// taskInstruction returns the instruction line shown above the question prompt.
func taskInstruction(t *task.Task) string {
	if t.Type == task.TypeGrammar {
		return "Выберите правильный вариант, чтобы заполнить пропуск:"
	}
	if t.Metadata == nil {
		return ""
	}
	switch t.Metadata["subtype"] {
	case task.VocabSubtypeRuEn:
		return "Выберите правильный перевод на английский язык:"
	case task.VocabSubtypeEnRu:
		return "Выберите правильный перевод на русский язык:"
	case task.VocabSubtypeFillBlank:
		return "Выберите правильное слово для заполнения пропуска:"
	case task.VocabSubtypeTypingEnRu:
		return "Введи правильный перевод на русский язык одним сообщением:"
	case task.VocabSubtypeTypingRuEn:
		return "Введи правильный перевод на английский язык одним сообщением:"
	}
	return ""
}

// vocabSubtype returns the metadata subtype string, or "" if absent.
func vocabSubtype(t *task.Task) string {
	if t.Metadata == nil {
		return ""
	}
	s, _ := t.Metadata["subtype"].(string)
	return s
}

func (b *Bot) sendTaskQuestion(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, t *task.Task, current, total int) {
	typeLabel := vocabTaskLabel(t)
	if t.Type == task.TypeGrammar {
		typeLabel = "📝 Грамматика"
	}

	instruction := taskInstruction(t)
	var text string
	if instruction != "" {
		text = fmt.Sprintf("%s — Задание %d/%d\n%s\n\n%s", typeLabel, current, total, instruction, t.Prompt)
	} else {
		text = fmt.Sprintf("%s — Задание %d/%d\n\n%s", typeLabel, current, total, t.Prompt)
	}

	subtype := vocabSubtype(t)
	isTyping := task.IsVocabTypingSubtype(subtype)

	buttons := make([][]models.InlineKeyboardButton, 0, len(t.Options)+1)
	if !isTyping {
		for i, opt := range t.Options {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{Text: opt, CallbackData: fmt.Sprintf("task:%d:%d", t.ID, i)},
			})
		}
	}
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "🤷 Не знаю", CallbackData: fmt.Sprintf("task:%d:idk", t.ID)},
	})

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})

	// For typing tasks, register a pending typing prompt so the user's next
	// text reply is routed to handleTasksTypingReply via defaultHandler.
	if isTyping && b.pendingRepo != nil {
		pending := &words.PendingTyping{
			UserID:    userID,
			ChatID:    chatID,
			Source:    "tasks",
			Payload:   words.PendingPayload{TaskID: t.ID},
			ExpiresAt: time.Now().Add(words.PendingTTL),
		}
		if err := b.pendingRepo.Upsert(ctx, pending); err != nil {
			slog.Error("failed to upsert pending typing for task", "user_id", userID, "task_id", t.ID, "error", err)
		}
	}
}

// handleTasksTypingReply is invoked when the user sends a text reply to a
// /tasks typing-vocab task. Mirrors handleTaskCallback's post-answer logic
// (feedback, audio, score recording, next task) but reads the answer from the
// raw user text instead of an option button.
func (b *Bot) handleTasksTypingReply(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, raw string, payload words.PendingPayload) {
	taskID := payload.TaskID
	if taskID == 0 {
		return
	}

	t, err := b.taskSvc.GetTaskByID(ctx, taskID)
	if err != nil {
		slog.Error("failed to load task for typing reply", "task_id", taskID, "error", err)
		return
	}

	result, err := b.taskSvc.SubmitTypedAnswer(ctx, taskID, userID, raw)
	if err != nil {
		if errors.Is(err, task.ErrAlreadyAnswered) {
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Ты уже ответил на это задание.",
			})
			return
		}
		slog.Error("failed to submit typed answer", "user_id", userID, "task_id", taskID, "error", err)
		return
	}

	slog.Info("task typing answered", "user_id", userID, "task_id", taskID, "level", t.Level, "correct", result.Correct, "score", result.Score)

	var feedback string
	correctPhrases := []string{"✅ Правильно!", "✅ Отлично!", "✅ Молодец!", "✅ В точку!", "✅ Супер!"}
	if result.Correct {
		feedback = correctPhrases[taskID%int64(len(correctPhrases))]
		if b.scoringSvc != nil {
			if err := b.scoringSvc.RecordScore(ctx, userID, t.GroupID, t.Level, result.Score); err != nil {
				slog.Error("failed to record score", "user_id", userID, "task_id", taskID, "error", err)
			}
		}
	} else {
		feedback = buildIncorrectFeedback(t)
	}

	feedbackParams := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      feedback,
		ParseMode: models.ParseModeHTML,
	}
	if t.Type == task.TypeVocab && t.Metadata != nil {
		if wordID := toInt64(t.Metadata["word_id"]); wordID > 0 {
			feedbackParams.ReplyMarkup = &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: "🔊 Послушать", CallbackData: fmt.Sprintf("word_audio:%d", wordID)}},
				},
			}
		}
	}
	tgBot.SendMessage(ctx, feedbackParams)

	// Move to next task or completion screen.
	nextTask, _ := b.taskSvc.GetNextUnanswered(ctx, t.DailySetID, userID)
	if nextTask == nil {
		_, total, _ := b.taskSvc.GetProgress(ctx, t.DailySetID, userID)
		correct, _ := b.taskSvc.GetCorrectCount(ctx, t.DailySetID, userID)
		slog.Info("round completed", "user_id", userID, "daily_set_id", t.DailySetID, "correct", correct, "total", total)
		var completionText string
		if correct == total {
			completionText = fmt.Sprintf("🏆 Все задания выполнены!\n\n✅ Правильных ответов: %d из %d\n\nТы огонь! Загляни в /leaderboard — посмотри, где ты в рейтинге 🔥", correct, total)
		} else {
			completionText = fmt.Sprintf("✅ Задания выполнены!\n\n✅ Правильных ответов: %d из %d\n\nЗагляни в /leaderboard — посмотри, где ты в рейтинге 🔥", correct, total)
		}
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   completionText,
		})
		return
	}

	answered, total, _ := b.taskSvc.GetProgress(ctx, t.DailySetID, userID)
	b.sendTaskQuestion(ctx, tgBot, chatID, userID, nextTask, answered+1, total)
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

// buildIncorrectFeedback returns the wrong-answer message for a task (HTML-formatted).
func buildIncorrectFeedback(t *task.Task) string {
	base := fmt.Sprintf("❌ Неправильно. Правильный ответ: <b>%s</b>", html.EscapeString(t.Answer))
	if t.Type != task.TypeGrammar || t.Metadata == nil {
		return base
	}
	explanation, _ := t.Metadata["explanation"].(string)
	if explanation == "" {
		return base
	}
	return base + "\n\n💡 " + formatExplanationHTML(explanation)
}

// buildSkipFeedback returns the message shown when the user taps "🤷 Не знаю".
// Mirrors buildIncorrectFeedback but leads with an honest-skip prefix.
func buildSkipFeedback(t *task.Task) string {
	base := fmt.Sprintf("🤷 Пропущено. Правильный ответ: <b>%s</b>", html.EscapeString(t.Answer))
	if t.Type != task.TypeGrammar || t.Metadata == nil {
		return base
	}
	explanation, _ := t.Metadata["explanation"].(string)
	if explanation == "" {
		return base
	}
	return base + "\n\n💡 " + formatExplanationHTML(explanation)
}

// formatExplanationHTML escapes plain-text explanation for HTML and bolds section headers
// (lines that end with ":" and are not bullet points).
func formatExplanationHTML(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 1 &&
			strings.HasSuffix(trimmed, ":") &&
			!strings.HasPrefix(trimmed, "•") &&
			!strings.HasPrefix(trimmed, "✗") &&
			!strings.HasPrefix(trimmed, "✓") {
			lines[i] = "<b>" + html.EscapeString(trimmed) + "</b>"
		} else {
			lines[i] = html.EscapeString(line)
		}
	}
	return strings.Join(lines, "\n")
}
