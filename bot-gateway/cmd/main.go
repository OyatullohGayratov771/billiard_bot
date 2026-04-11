package main

import (
	"log"
	"net/http"
	"time"

	"bot-gateway/internal/bot"
	"bot-gateway/internal/client"
	"bot-gateway/internal/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const workerCount = 64 // bir vaqtda max 64 update

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	// Timeout va connection pool bilan HTTP client
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// HTTP klientlar orqali mikroservislar bilan ulanish
	userClient := client.NewUserClient(cfg.UserServiceURL, httpClient)
	tableClient := client.NewTableClient(cfg.TableServiceURL, httpClient)
	clipClient := client.NewClipClient(cfg.ClipServiceURL, httpClient)

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

	// Worker pool — cheksiz goroutine o'rniga limit
	sem := make(chan struct{}, workerCount)
	for update := range updates {
		upd := update
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			handler.Handle(tg, upd)
		}()
	}
}
