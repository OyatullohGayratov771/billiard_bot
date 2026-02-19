package main

import (
	"log"

	"bot-gateway/internal/bot"
	"bot-gateway/internal/config"
	gw "bot-gateway/internal/grpc"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	userClient, err := gw.NewUserClient(cfg.UserGRPCAddr, cfg.APIKey)
	if err != nil {
		log.Fatal(err)
	}

	tg, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal(err)
	}

	handler := bot.NewHandler(userClient)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tg.GetUpdatesChan(u)

	log.Println("🤖 bot-gateway started")

	for update := range updates {
		handler.Handle(tg, update)
	}
}
