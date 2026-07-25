package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/scoring"
)

func (b *Bot) handleProfile(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	slog.Debug("cmd received", "cmd", "/profile", "user_id", u.ID)

	if !u.Onboarded {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "🎓 Сначала пройди оценку уровня, команда: /assess",
		})
		return
	}

	monthKey := time.Now().Format("2006-01")
	monthlyScore, err := b.scoringSvc.GetMonthlyScore(ctx, u.ID, monthKey)
	if err != nil {
		slog.Error("failed to get monthly score", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось загрузить профиль. Попробуй позже.",
		})
		return
	}

	scoreDays, err := b.scoringSvc.GetScoreDays(ctx, u.ID)
	if err != nil {
		slog.Error("failed to get score days", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось загрузить профиль. Попробуй позже.",
		})
		return
	}

	learnedWords, err := b.srsRepo.CountLearnedWords(ctx, u.ID)
	if err != nil {
		slog.Error("failed to count learned words", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось загрузить профиль. Попробуй позже.",
		})
		return
	}

	var inProgressWords, learnedViaWords int
	if b.wordsSvc != nil {
		if cnt, err := b.wordsSvc.CountInProgress(ctx, u.ID); err == nil {
			inProgressWords = cnt
		}
		if cnt, err := b.wordsSvc.CountLearnedViaWords(ctx, u.ID); err == nil {
			learnedViaWords = cnt
		}
	}

	streak := scoring.Streak(scoreDays, time.Now())

	slog.Info("profile viewed", "user_id", u.ID, "level", u.Level, "streak", streak, "monthly_score", monthlyScore, "learned_words", learnedWords, "in_progress_words", inProgressWords, "learned_via_words", learnedViaWords)

	var streakLine string
	switch {
	case streak == 0:
		streakLine = "🎽 Дней подряд занимаешься: 0 — самое время начать серию! 💪"
	case streak < 3:
		streakLine = fmt.Sprintf("🎽 Дней подряд занимаешься: %d 🔥 — хорошее начало!", streak)
	case streak < 7:
		streakLine = fmt.Sprintf("🎽 Дней подряд занимаешься: %d 🔥 — отличный ритм!", streak)
	default:
		streakLine = fmt.Sprintf("🎽 Дней подряд занимаешься: %d 🔥🔥🔥 — ты в огне!", streak)
	}

	var sb strings.Builder

	// Identity
	sb.WriteString(fmt.Sprintf("🎓 Уровень: %s\n\n", u.Level))

	// Leaderboard-relevant stats: monthly score and the daily activity streak
	// belong together — both feed the user's standing in their group.
	sb.WriteString(fmt.Sprintf("⭐ Баллы за месяц: %.1f баллов\n", monthlyScore))
	sb.WriteString(streakLine + "\n\n")

	// Vocabulary section: aggregate counts under one header.
	sb.WriteString("📚 Слова\n")
	sb.WriteString(fmt.Sprintf("🧠 В интервальных повторениях: %d (стабильно — 3+ повторений)\n", learnedWords))
	if learnedViaWords > 0 || inProgressWords > 0 {
		sb.WriteString(fmt.Sprintf("✍️ Усвоено через /words: %d\n", learnedViaWords))
		sb.WriteString(fmt.Sprintf("📖 В работе: %d (продолжай — /words)\n", inProgressWords))
	}

	// Per-level breakdown — separate paragraph since it's a different shape
	// (per-CEFR-level rows vs the aggregate counts above).
	if b.wordsSvc != nil {
		if levels, err := b.wordsSvc.LevelProgress(ctx, u.ID); err == nil {
			if block := formatLevelProgress(levels); block != "" {
				sb.WriteString("\n")
				sb.WriteString(block)
			}
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
	})
}

// calcStreak counts consecutive days up to and including today where the user has scores.
