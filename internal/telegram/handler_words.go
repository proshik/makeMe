package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/srs"
	"makeMe/internal/domain/words"
)

// handleWords is the entry point for the /words command. It opens (or resumes)
// a learning session and immediately shows the first slot. /words sessions do
// NOT contribute to the leaderboard — see handleWordsAnswered for the absence
// of scoringSvc.RecordScore.
func (b *Bot) handleWords(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	u := UserFromContext(ctx)
	if u == nil {
		return
	}
	slog.Debug("cmd received", "cmd", "/words", "user_id", u.ID)

	if !u.Onboarded {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "🎓 Сначала пройди тест на определение уровня: /assess",
		})
		return
	}

	chatID := update.Message.Chat.ID
	session, _, resumed, err := b.wordsSvc.StartOrResumeSession(ctx, u.ID)
	if err != nil {
		slog.Error("failed to start words session", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "😔 Не удалось загрузить слова для изучения. Попробуй позже.",
		})
		return
	}

	view, err := b.wordsSvc.Status(ctx, u.ID, session)
	if err != nil {
		slog.Warn("failed to compute status", "user_id", u.ID, "error", err)
	}

	intro := fmt.Sprintf(
		"📚 <b>Учим слова</b>\n\n"+
			"Слов в сессии: %d\nПрогресс: %d/%d показов\nВсего выучено: %d\n\n"+
			"<i>Каждое слово учится в 4 режимах: EN→RU и RU→EN с выбором, плюс ввод полного перевода в обе стороны.</i>",
		view.WordsInSession, view.ModesCompleted, view.ModesTotal, view.LearnedAllTime,
	)
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      intro,
		ParseMode: models.ParseModeHTML,
	})

	if resumed {
		slog.Info("words session resumed", "user_id", u.ID, "session_id", session.ID, "level", u.Level, "size", len(session.WordIDs), "modes_completed", view.ModesCompleted, "modes_total", view.ModesTotal)
	} else {
		slog.Info("words session created", "user_id", u.ID, "session_id", session.ID, "level", u.Level, "size", len(session.WordIDs))
	}

	b.advanceWordsSlot(ctx, tgBot, chatID, u.ID, session.ID)
}

// handleWordsCallback dispatches words: callbacks.
func (b *Bot) handleWordsCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
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
	case data == "words:stop":
		b.handleWordsStop(ctx, tgBot, chatID, u.ID)
	case strings.HasPrefix(data, "words:mc:"):
		b.handleWordsMC(ctx, tgBot, chatID, u.ID, data)
	case strings.HasPrefix(data, "words:idk:"):
		b.handleWordsIdk(ctx, tgBot, chatID, u.ID, data)
	case strings.HasPrefix(data, "words:know:"):
		b.handleWordsKnow(ctx, tgBot, chatID, u.ID, data)
	case strings.HasPrefix(data, "words:defer:"):
		b.handleWordsDefer(ctx, tgBot, chatID, u.ID, data)
	}
}

// handleWordsMC parses "words:mc:<sid>:<wid>:<modeCode>:<chosenIdx>:<correctIdx>".
func (b *Bot) handleWordsMC(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, data string) {
	sid, wid, mode, chosenIdx, correctIdx, ok := parseWordsMC(data)
	if !ok {
		return
	}
	session, err := b.requireSession(ctx, sid)
	if err != nil {
		slog.Error("failed to load session", "session_id", sid, "error", err)
		return
	}
	correct := chosenIdx == correctIdx
	out, err := b.wordsSvc.SubmitMC(ctx, session, wid, mode, correct)
	if err != nil {
		slog.Error("failed to submit MC answer", "user_id", userID, "session_id", sid, "word_id", wid, "error", err)
		return
	}
	slog.Info("words mc answered", "user_id", userID, "session_id", sid, "word_id", wid, "mode", mode, "correct", correct, "became_learned", out.BecameLearned)

	b.sendWordsFeedback(ctx, tgBot, chatID, session, wid, out, false)
	b.advanceWordsSlot(ctx, tgBot, chatID, userID, sid)
}

// handleWordsIdk parses "words:idk:<sid>:<wid>:<modeCode>".
// Treated as a wrong answer (resets the per-mode counter to 0).
func (b *Bot) handleWordsIdk(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, data string) {
	sid, wid, mode, ok := parseWordsIdk(data)
	if !ok {
		return
	}
	session, err := b.requireSession(ctx, sid)
	if err != nil {
		slog.Error("failed to load session", "session_id", sid, "error", err)
		return
	}
	// Clear any pending typing prompt — user chose "Не знаю" instead of typing.
	_ = b.pendingRepo.Delete(ctx, userID)

	out, err := b.wordsSvc.SubmitSkip(ctx, session, wid, mode)
	if err != nil {
		slog.Error("failed to submit skip", "user_id", userID, "session_id", sid, "word_id", wid, "error", err)
		return
	}
	slog.Info("words skipped", "user_id", userID, "session_id", sid, "word_id", wid, "mode", mode)

	b.sendWordsFeedback(ctx, tgBot, chatID, session, wid, out, true)
	b.advanceWordsSlot(ctx, tgBot, chatID, userID, sid)
}

// handleWordsKnow parses "words:know:<sid>:<wid>". Marks the word as learned
// across all 4 modes and creates a SM-2 review_card so it joins the spaced
// repetition cycle.
func (b *Bot) handleWordsKnow(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, data string) {
	sid, wid, ok := parseWordsKnow(data)
	if !ok {
		return
	}
	session, err := b.requireSession(ctx, sid)
	if err != nil {
		slog.Error("failed to load session", "session_id", sid, "error", err)
		return
	}
	if err := b.wordsSvc.MarkAlreadyKnown(ctx, session, wid); err != nil {
		slog.Error("failed to mark already known", "user_id", userID, "word_id", wid, "error", err)
		return
	}
	slog.Info("words marked known", "user_id", userID, "session_id", sid, "word_id", wid)

	word, err := b.srsRepo.GetWordByID(ctx, wid)
	if err == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("✅ Ок, считаем что ты знаешь слово <b>%s</b> — <i>%s</i>. Слово ушло в интервальные повторения.", html.EscapeString(word.English), html.EscapeString(word.Russian)),
			ParseMode: models.ParseModeHTML,
		})
	}

	b.advanceWordsSlot(ctx, tgBot, chatID, userID, sid)
}

// handleWordsDefer parses "words:defer:<sid>:<wid>" and removes the word from
// the active session without marking it learned. Triggered by the
// "⏭ Отложить это слово до завтра" button shown in feedback after >5 mistakes.
func (b *Bot) handleWordsDefer(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, data string) {
	sid, wid, ok := parseWordsDefer(data)
	if !ok {
		return
	}
	session, err := b.requireSession(ctx, sid)
	if err != nil || session == nil {
		slog.Error("failed to load session", "session_id", sid, "error", err)
		return
	}
	if err := b.wordsSvc.DeferWord(ctx, session, wid); err != nil {
		slog.Error("failed to defer word", "user_id", userID, "session_id", sid, "word_id", wid, "error", err)
		return
	}
	slog.Info("words deferred", "user_id", userID, "session_id", sid, "word_id", wid)

	word, err := b.srsRepo.GetWordByID(ctx, wid)
	confirmation := "📌 Слово отложено. Прогресс сохранён, оно вернётся в следующей сессии /words."
	if err == nil {
		confirmation = fmt.Sprintf("📌 Слово <b>%s</b> отложено. Прогресс сохранён, оно вернётся в следующей сессии /words.", html.EscapeString(word.English))
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      confirmation,
		ParseMode: models.ParseModeHTML,
	})

	// Reload session so DeferredWordIDs reflects the new state for NextSlot.
	b.advanceWordsSlot(ctx, tgBot, chatID, userID, sid)
}

// handleWordsStop ends the current session and shows a brief summary.
func (b *Bot) handleWordsStop(ctx context.Context, tgBot *bot.Bot, chatID, userID int64) {
	session, err := b.wordsSvc.RepoOpenSession(ctx, userID)
	if err != nil || session == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Активной сессии нет."})
		return
	}
	view, _ := b.wordsSvc.Status(ctx, userID, session)
	if err := b.wordsSvc.CompleteSession(ctx, session.ID); err != nil {
		slog.Error("failed to complete session", "session_id", session.ID, "error", err)
	}
	slog.Info("words session stopped", "user_id", userID, "session_id", session.ID,
		"learned_in_session", view.LearnedThisRound, "size", view.WordsInSession,
		"modes_completed", view.ModesCompleted, "modes_total", view.ModesTotal)
	b.sendWordsCompletion(ctx, tgBot, chatID, userID, session)
}

// advanceWordsSlot loads the next slot and either renders it or shows the
// completion screen if the session is done.
func (b *Bot) advanceWordsSlot(ctx context.Context, tgBot *bot.Bot, chatID, userID, sessionID int64) {
	slot, session, done, err := b.wordsSvc.NextSlot(ctx, sessionID)
	if err != nil {
		slog.Error("failed to compute next slot", "session_id", sessionID, "error", err)
		return
	}
	if done {
		view, _ := b.wordsSvc.Status(ctx, userID, session)
		slog.Info("words session completed", "user_id", userID, "session_id", session.ID,
			"learned_in_session", view.LearnedThisRound, "size", view.WordsInSession,
			"modes_completed", view.ModesCompleted, "modes_total", view.ModesTotal)
		b.sendWordsCompletion(ctx, tgBot, chatID, userID, session)
		return
	}

	view, _ := b.wordsSvc.Status(ctx, userID, session)
	slog.Debug("words slot shown", "user_id", userID, "session_id", session.ID,
		"word_id", slot.Word.ID, "mode", slot.Mode, "modes_completed", view.ModesCompleted, "modes_total", view.ModesTotal)
	b.sendWordsSlot(ctx, tgBot, chatID, userID, session, slot, view)
}

// sendWordsSlot renders one (word, mode) prompt with appropriate buttons.
// MC modes: 4 option buttons + "🤷 Не знаю" + (optionally) "Я уже знаю это слово".
// Typing modes: only "🤷 Не знаю" + (optionally) "Я уже знаю", plus a pending
// typing prompt is registered in the DB so the user's next text reply is
// routed back here.
//
// The header deliberately omits the word's position in the session (e.g.
// "слово 9/10") — that would be a memorization-aid leak, letting the user
// recognize a hard word by its serial number.
func (b *Bot) sendWordsSlot(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, session *words.Session, slot *words.Slot, view words.StatusView) {
	header := fmt.Sprintf("📖 %s · показ %d/%d", modeLabel(slot.Mode), view.ModesCompleted, view.ModesTotal)

	progress, _ := b.wordsSvc.RepoGetProgress(ctx, userID, slot.Word.ID)
	showKnowButton := progress == nil

	switch slot.Mode {
	case words.ModeEnRuMC, words.ModeRuEnMC:
		b.sendWordsMCSlot(ctx, tgBot, chatID, session, slot, header, showKnowButton)
	case words.ModeEnRuTyping, words.ModeRuEnTyping:
		b.sendWordsTypingSlot(ctx, tgBot, chatID, userID, session, slot, header, showKnowButton)
	}
}

func (b *Bot) sendWordsMCSlot(ctx context.Context, tgBot *bot.Bot, chatID int64, session *words.Session, slot *words.Slot, header string, showKnow bool) {
	correct, prompt := mcPromptAndAnswer(slot)

	// Distractors: words at the same level as the slot's word.
	distractors, err := b.srsRepo.GetWordsByLevel(ctx, slot.Word.Level, 10)
	if err != nil {
		slog.Error("failed to get distractors", "level", slot.Word.Level, "error", err)
		return
	}
	pick := russianPicker
	if slot.Mode == words.ModeRuEnMC {
		pick = englishPicker
	}
	options, correctIdx := buildMCOptions(correct, distractors, pick)

	modeCode := words.EncodeMode(slot.Mode)
	buttons := make([][]models.InlineKeyboardButton, 0, len(options)+2)
	for i, opt := range options {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         opt,
				CallbackData: fmt.Sprintf("words:mc:%d:%d:%s:%d:%d", session.ID, slot.Word.ID, modeCode, i, correctIdx),
			},
		})
	}
	buttons = append(buttons, []models.InlineKeyboardButton{
		{
			Text:         "🤷 Не знаю",
			CallbackData: fmt.Sprintf("words:idk:%d:%d:%s", session.ID, slot.Word.ID, modeCode),
		},
	})
	if showKnow {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         "✅ Я уже знаю это слово",
				CallbackData: fmt.Sprintf("words:know:%d:%d", session.ID, slot.Word.ID),
			},
		})
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("%s\n\n%s", header, prompt),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func (b *Bot) sendWordsTypingSlot(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, session *words.Session, slot *words.Slot, header string, showKnow bool) {
	prompt := typingPrompt(slot)

	modeCode := words.EncodeMode(slot.Mode)
	buttons := [][]models.InlineKeyboardButton{
		{{
			Text:         "🤷 Не знаю",
			CallbackData: fmt.Sprintf("words:idk:%d:%d:%s", session.ID, slot.Word.ID, modeCode),
		}},
	}
	if showKnow {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         "✅ Я уже знаю это слово",
				CallbackData: fmt.Sprintf("words:know:%d:%d", session.ID, slot.Word.ID),
			},
		})
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("%s\n\n%s", header, prompt),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})

	// Register a pending typing prompt: the user's next text reply will be
	// routed back through defaultHandler → handleTypingReply.
	pending := &words.PendingTyping{
		UserID:    userID,
		ChatID:    chatID,
		Source:    "words",
		Payload:   words.PendingPayload{SessionID: session.ID, WordID: slot.Word.ID, Mode: slot.Mode},
		ExpiresAt: time.Now().Add(words.PendingTTL),
	}
	if err := b.pendingRepo.Upsert(ctx, pending); err != nil {
		slog.Error("failed to upsert pending typing", "user_id", userID, "error", err)
	}
}

// sendWordsFeedback shows the result of a single answer (correct/incorrect/
// skipped) plus optional follow-up buttons (audio, "defer until tomorrow").
//
// The defer button is shown only on a wrong/skipped answer when the user has
// already accumulated more than DeferThreshold mistakes on this word in the
// current session — it's an escape hatch for words that just won't stick.
func (b *Bot) sendWordsFeedback(ctx context.Context, tgBot *bot.Bot, chatID int64, session *words.Session, wordID int64, out words.Outcome, skipped bool) {
	word, err := b.srsRepo.GetWordByID(ctx, wordID)
	if err != nil {
		slog.Error("failed to get word for feedback", "word_id", wordID, "error", err)
		return
	}

	var text string
	switch {
	case skipped:
		text = fmt.Sprintf("🤷 Правильный ответ: <b>%s</b>", html.EscapeString(out.Expected))
	case out.Correct:
		text = "✅ Правильно!"
	default:
		text = fmt.Sprintf("❌ Неправильно. Правильный ответ: <b>%s</b>", html.EscapeString(out.Expected))
	}
	if word.Example != "" {
		text += fmt.Sprintf("\n\n<i>%s</i>", html.EscapeString(word.Example))
	}

	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}

	// Audio button is always offered: handleWordAudioCallback lazily fetches
	// pronunciation from the Free Dictionary API on first click and falls back
	// to a "not found" reply if the word has no audio. Without this, the
	// button would never appear for words whose audio_url has not yet been
	// populated — circular and unhelpful.
	rows := [][]models.InlineKeyboardButton{
		{{Text: "🔊 Послушать", CallbackData: fmt.Sprintf("word_audio:%d", wordID)}},
	}
	if !out.Correct && session != nil && out.ErrorsInSession > words.DeferThreshold {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "⏭ Отложить это слово до завтра",
				CallbackData: fmt.Sprintf("words:defer:%d:%d", session.ID, wordID)},
		})
	}
	params.ReplyMarkup = &models.InlineKeyboardMarkup{InlineKeyboard: rows}

	tgBot.SendMessage(ctx, params)

	if out.BecameLearned {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("🎉 Слово <b>%s</b> выучено! Ушло в интервальные повторения, дальше встретится в /tasks.", html.EscapeString(word.English)),
			ParseMode: models.ParseModeHTML,
		})
	}
}

func (b *Bot) sendWordsCompletion(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, session *words.Session) {
	if session == nil {
		return
	}
	view, _ := b.wordsSvc.Status(ctx, userID, session)
	text := fmt.Sprintf(
		"🎉 <b>Сессия завершена!</b>\n\nВыучено в этой сессии: %d из %d\nВсего выучено через /words: %d\n\nХочешь ещё? Запусти /words снова — дам новый набор.",
		view.LearnedThisRound, view.WordsInSession, view.LearnedAllTime,
	)
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
}

// handleWordsTypingReply is called from the typing dispatcher when the user
// types a reply to a /words typing prompt.
func (b *Bot) handleWordsTypingReply(ctx context.Context, tgBot *bot.Bot, chatID, userID int64, raw string, payload words.PendingPayload) {
	session, err := b.requireSession(ctx, payload.SessionID)
	if err != nil || session == nil {
		slog.Warn("words typing reply for missing session", "session_id", payload.SessionID, "error", err)
		return
	}
	if session.CompletedAt != nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Сессия уже завершена. Запусти /words чтобы начать новую.",
		})
		return
	}

	out, err := b.wordsSvc.SubmitTyping(ctx, session, payload.WordID, payload.Mode, raw)
	if err != nil {
		slog.Error("failed to submit typing answer", "user_id", userID, "error", err)
		return
	}
	slog.Info("words typing answered", "user_id", userID, "session_id", session.ID, "word_id", payload.WordID, "mode", payload.Mode, "correct", out.Correct, "became_learned", out.BecameLearned)

	b.sendWordsFeedback(ctx, tgBot, chatID, session, payload.WordID, out, false)
	b.advanceWordsSlot(ctx, tgBot, chatID, userID, session.ID)
}

// requireSession is a thin wrapper that returns ErrSessionNotFound if missing.
func (b *Bot) requireSession(ctx context.Context, sessionID int64) (*words.Session, error) {
	return b.wordsSvc.RepoGetSessionByID(ctx, sessionID)
}

// --- helpers ---

func modeLabel(m words.Mode) string {
	switch m {
	case words.ModeEnRuMC:
		return "EN→RU выбор"
	case words.ModeRuEnMC:
		return "RU→EN выбор"
	case words.ModeEnRuTyping:
		return "EN→RU ввод"
	case words.ModeRuEnTyping:
		return "RU→EN ввод"
	}
	return string(m)
}

// mcPromptAndAnswer returns the prompt HTML and the correct option text.
func mcPromptAndAnswer(slot *words.Slot) (correct, prompt string) {
	w := slot.Word
	switch slot.Mode {
	case words.ModeEnRuMC:
		header := html.EscapeString(w.English)
		if w.Transcription != "" {
			header += "  " + html.EscapeString(w.Transcription)
		}
		body := "Выбери правильный перевод на русский:\n\n" + header
		if w.Example != "" {
			body += "\n\n<i>" + html.EscapeString(w.Example) + "</i>"
		}
		return w.Russian, body
	case words.ModeRuEnMC:
		body := "Выбери правильный перевод на английский:\n\n" + html.EscapeString(w.Russian)
		return w.English, body
	}
	return "", ""
}

// typingPrompt returns the prompt HTML for a typing slot. The trailing line
// makes the instruction explicit ("type a reply") because the message has no
// answer buttons and users tend to wait for them.
func typingPrompt(slot *words.Slot) string {
	w := slot.Word
	const tail = "\n\n<i>💬 Напиши перевод одним сообщением в чат.</i>"
	switch slot.Mode {
	case words.ModeEnRuTyping:
		header := html.EscapeString(w.English)
		if w.Transcription != "" {
			header += "  " + html.EscapeString(w.Transcription)
		}
		body := "✍️ <b>Введи перевод на русский</b>\n\n" + header
		if w.Example != "" {
			body += "\n\n<i>" + html.EscapeString(w.Example) + "</i>"
		}
		return body + tail
	case words.ModeRuEnTyping:
		return "✍️ <b>Введи перевод на английский</b>\n\n" + html.EscapeString(w.Russian) + tail
	}
	return ""
}

// buildMCOptions assembles 4 shuffled options (1 correct + 3 distractors) and
// returns both the option list and the index of the correct one.
func buildMCOptions(correct string, distractors []srs.Word, pick func(srs.Word) string) ([]string, int) {
	options := []string{correct}
	for _, d := range distractors {
		v := pick(d)
		if v == correct {
			continue
		}
		options = append(options, v)
		if len(options) >= 4 {
			break
		}
	}
	rand.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})
	correctIdx := 0
	for i, o := range options {
		if o == correct {
			correctIdx = i
			break
		}
	}
	return options, correctIdx
}

func russianPicker(w srs.Word) string { return w.Russian }
func englishPicker(w srs.Word) string { return w.English }

// --- callback parsers ---

// parseWordsMC parses "words:mc:<sid>:<wid>:<modeCode>:<chosenIdx>:<correctIdx>".
func parseWordsMC(data string) (sid, wid int64, mode words.Mode, chosenIdx, correctIdx int, ok bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 7 {
		return 0, 0, "", 0, 0, false
	}
	var err error
	sid, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, "", 0, 0, false
	}
	wid, err = strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, 0, "", 0, 0, false
	}
	mode, ok = words.DecodeMode(parts[4])
	if !ok {
		return 0, 0, "", 0, 0, false
	}
	chosenIdx, err = strconv.Atoi(parts[5])
	if err != nil {
		return 0, 0, "", 0, 0, false
	}
	correctIdx, err = strconv.Atoi(parts[6])
	if err != nil {
		return 0, 0, "", 0, 0, false
	}
	return sid, wid, mode, chosenIdx, correctIdx, true
}

func parseWordsIdk(data string) (sid, wid int64, mode words.Mode, ok bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 5 {
		return 0, 0, "", false
	}
	var err error
	sid, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	wid, err = strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	mode, ok = words.DecodeMode(parts[4])
	if !ok {
		return 0, 0, "", false
	}
	return sid, wid, mode, true
}

func parseWordsKnow(data string) (sid, wid int64, ok bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 4 {
		return 0, 0, false
	}
	var err error
	sid, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	wid, err = strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return sid, wid, true
}

// parseWordsDefer parses "words:defer:<sid>:<wid>".
func parseWordsDefer(data string) (sid, wid int64, ok bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 4 {
		return 0, 0, false
	}
	var err error
	sid, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	wid, err = strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return sid, wid, true
}
