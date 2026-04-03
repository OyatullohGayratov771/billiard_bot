package main

import (
	"log"

	"bot-gateway/internal/bot"
	"bot-gateway/internal/client"
	"bot-gateway/internal/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	// HTTP klientlar orqali mikroservislar bilan ulanish
	userClient := client.NewUserClient(cfg.UserServiceURL)
	tableClient := client.NewTableClient(cfg.TableServiceURL)
	clipClient := client.NewClipClient(cfg.ClipServiceURL)

	// Handler
	handler := bot.NewHandler(userClient, tableClient, clipClient)

	// Telegram bot
	tg, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("❌ Bot yaratilmadi: %v", err)
	}

	log.Printf("🤖 Bot @%s ishga tushdi", tg.Self.UserName)

	// Long polling
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tg.GetUpdatesChan(u)

	for update := range updates {
		go handler.Handle(tg, update)
	}
}
