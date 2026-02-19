package bot

import (
	"context"
	"fmt"
	"log"

	"bot-gateway/internal/grpc"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	user *grpc.UserClient
}

func NewHandler(user *grpc.UserClient) *Handler {
	return &Handler{user: user}
}

func (h *Handler) Handle(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	chatID := msg.Chat.ID

	if msg.IsCommand() && msg.Command() == "start" {
		user, err := h.user.Register(
			context.Background(),
			msg.From.ID,
			msg.From.UserName,
			msg.From.FirstName,
			msg.From.LastName,
		)

		if err != nil {
			log.Println("register error:", err)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Registration failed"))
			return
		}

		text := "✅ Registered!\n" +
			"ID: %d\nUsername: @%s\nRole: %v"

		bot.Send(tgbotapi.NewMessage(
			chatID,
			fmt.Sprintf(text, user.Id, user.Username, user.Role),
		))
	}
}
