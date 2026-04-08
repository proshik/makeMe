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

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: fmt.Sprintf(
			"Группа «%s» создана!\n\nИнвайт-код: %s\nОтправь его друзьям — они смогут вступить командой:\n/join %s",
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

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Ты вступил в группу «%s»!", g.Name),
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
			Text:   "У тебя пока нет групп.\nСоздай новую: /newgroup Название\nИли вступи по коду: /join КОД",
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
