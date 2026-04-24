package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"makeMe/internal/domain/group"
)

func (b *Bot) handleNewGroup(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	slog.Debug("cmd received", "cmd", "/newgroup", "user_id", u.ID)

	name := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/newgroup"))
	if name == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Укажи название группы: /newgroup Название",
		})
		return
	}

	g, err := b.groupSvc.Create(ctx, name, u.ID)
	if err != nil {
		slog.Error("failed to create group", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось создать группу. Попробуй позже.",
		})
		return
	}

	slog.Info("group created", "user_id", u.ID, "group_id", g.ID, "name", g.Name)

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: fmt.Sprintf(
			"🎉 Группа «%s» создана!\n\n"+
				"📨 Инвайт-код: `%s`\n"+
				"Отправь его друзьям — они вступят командой:\n/join %s\n\n"+
				"Когда все соберутся, начинайте: /tasks 💪",
			g.Name, g.InviteCode, g.InviteCode,
		),
	})
}

func (b *Bot) handleJoin(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	slog.Debug("cmd received", "cmd", "/join", "user_id", u.ID)

	code := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/join"))
	if code == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Укажи инвайт-код: /join КОД",
		})
		return
	}

	g, err := b.groupSvc.Join(ctx, code, u.ID)
	if err != nil {
		var text string
		switch {
		case errors.Is(err, group.ErrNotFound):
			text = "Группа с таким кодом не найдена."
		case errors.Is(err, group.ErrAlreadyMember):
			text = "Ты уже состоишь в этой группе."
		default:
			slog.Error("failed to join group", "user_id", u.ID, "code", code, "error", err)
			text = "Не удалось вступить в группу. Попробуй позже."
		}
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   text,
		})
		return
	}

	slog.Info("group joined", "user_id", u.ID, "group_id", g.ID, "name", g.Name)

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("🙌 Ты вступил в группу «%s»! Добро пожаловать в команду!\n\nНачинай соревноваться: /tasks 🚀", g.Name),
	})
}

func (b *Bot) handleMyGroups(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	u := UserFromContext(ctx)
	if u == nil {
		return
	}

	slog.Debug("cmd received", "cmd", "/mygroups", "user_id", u.ID)

	groups, err := b.groupSvc.ListByUser(ctx, u.ID)
	if err != nil {
		slog.Error("failed to list groups", "user_id", u.ID, "error", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось загрузить группы. Попробуй позже.",
		})
		return
	}

	if len(groups) == 0 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "👥 У тебя пока нет групп — позови друзей!\n\nСоздай новую: /newgroup Название\nВступи по коду: /join КОД",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("Твои группы:\n\n")
	for i, g := range groups {
		sb.WriteString(fmt.Sprintf("%d. %s (код: %s)\n", i+1, g.Name, g.InviteCode))
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   sb.String(),
	})
}
