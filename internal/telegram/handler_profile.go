package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) handleProfile(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
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

	streak := calcStreak(scoreDays)

	var streakLine string
	switch {
	case streak == 0:
		streakLine = "Дней подряд занимаешься: 0 — самое время начать серию! 💪"
	case streak < 3:
		streakLine = fmt.Sprintf("Дней подряд занимаешься: %d 🔥 — хорошее начало!", streak)
	case streak < 7:
		streakLine = fmt.Sprintf("Дней подряд занимаешься: %d 🔥 — отличный ритм!", streak)
	default:
		streakLine = fmt.Sprintf("Дней подряд занимаешься: %d 🔥🔥🔥 — ты в огне!", streak)
	}

	var sb strings.Builder
	sb.WriteString("👤 Твой профиль\n\n")
	sb.WriteString(fmt.Sprintf("🎓 Уровень: %s\n", u.Level))
	sb.WriteString(fmt.Sprintf("⭐ Баллы за месяц: %.1f баллов\n", monthlyScore))
	sb.WriteString(streakLine + "\n")
	sb.WriteString(fmt.Sprintf("📚 Выучено слов: %d (слово считается выученным после 3 повторений)\n", learnedWords))

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
	})
}

// calcStreak counts consecutive days up to and including today where the user has scores.
func calcStreak(days []string) int {
	set := make(map[string]bool, len(days))
	for _, d := range days {
		set[d] = true
	}
	streak := 0
	for d := time.Now(); ; d = d.AddDate(0, 0, -1) {
		if !set[d.Format("2006-01-02")] {
			break
		}
		streak++
	}
	return streak
}
