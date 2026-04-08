package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/group"
)

func (b *Bot) handleLeaderboard(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil {
		slog.Error("failed to get groups for leaderboard", "user_id", u.ID, "error", err)
		return
	}
	if len(groups) == 0 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "У тебя нет групп. Создай группу (/newgroup) или вступи в существующую (/join).",
		})
		return
	}

	if len(groups) == 1 {
		periodKey := time.Now().Format("2006-01-02")
		b.sendLeaderboard(ctx, tgBot, update.Message.Chat.ID, groups[0], "day", periodKey)
		return
	}

	periodKey := time.Now().Format("2006-01-02")
	buttons := make([][]models.InlineKeyboardButton, len(groups))
	for i, g := range groups {
		buttons[i] = []models.InlineKeyboardButton{
			{Text: g.Name, CallbackData: fmt.Sprintf("lb:%d:day:%s", g.ID, periodKey)},
		}
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Выбери группу:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func (b *Bot) handleLeaderboardCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	// Parse "lb:{groupID}:{period}:{periodKey}"
	parts := strings.SplitN(update.CallbackQuery.Data, ":", 4)
	if len(parts) != 4 {
		return
	}

	groupID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}
	period := parts[2]
	periodKey := parts[3]

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil {
		slog.Error("failed to get groups for leaderboard", "user_id", u.ID, "error", err)
		return
	}

	var selected *group.Group
	for _, g := range groups {
		if g.ID == groupID {
			selected = &g
			break
		}
	}
	if selected == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	b.sendLeaderboard(ctx, tgBot, chatID, *selected, period, periodKey)
}

func (b *Bot) sendLeaderboard(ctx context.Context, tgBot *bot.Bot, chatID int64, g group.Group, period, periodKey string) {
	entries, err := b.scoringSvc.GetLeaderboard(ctx, g.ID, period, periodKey)
	if err != nil {
		slog.Error("failed to get leaderboard", "group_id", g.ID, "error", err)
		return
	}

	var periodLabel string
	if period == "day" {
		periodLabel = "сегодня"
	} else {
		periodLabel = fmt.Sprintf("месяц %s", periodKey)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🏆 Группа «%s» — %s\n\n", g.Name, periodLabel))
	if len(entries) == 0 {
		sb.WriteString("Пока нет результатов.")
	} else {
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("%d. %s — %.1f очков\n", e.Rank, e.FirstName, e.Normalized))
		}
	}

	var toggleText, togglePeriod, toggleKey string
	if period == "day" {
		toggleText = "📅 За месяц"
		togglePeriod = "month"
		toggleKey = time.Now().Format("2006-01")
	} else {
		toggleText = "📅 За сегодня"
		togglePeriod = "day"
		toggleKey = time.Now().Format("2006-01-02")
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: toggleText, CallbackData: fmt.Sprintf("lb:%d:%s:%s", g.ID, togglePeriod, toggleKey)}},
			},
		},
	})
}
