package main

import (
	"log"
	"net/http"
	"runtime/debug"
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
	tournamentClient := client.NewTournamentClient(cfg.TournamentServiceURL, httpClient, cfg.InternalToken)
	productClient := client.NewProductClient(cfg.ShopServiceURL, httpClient, cfg.InternalToken)

	// Handler
	handler := bot.NewHandler(userClient, tableClient, clipClient, tournamentClient, productClient)

	// Telegram bot
	tg, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("❌ Bot yaratilmadi: %v", err)
	}

	log.Printf("🤖 Bot @%s ishga tushdi", tg.Self.UserName)

	// Telegram "Menu" tugmasida ko'rinadigan buyruqlar ro'yxati
	cmdCfg := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "🏠 Bosh menyu"},
		tgbotapi.BotCommand{Command: "help", Description: "ℹ️ Yordam"},
		tgbotapi.BotCommand{Command: "cancel", Description: "❌ Amalni bekor qilish"},
	)
	if _, err := tg.Request(cmdCfg); err != nil {
		log.Printf("setMyCommands xato: %v", err)
	}

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
			// Bitta xato update butun botni o'ldirmasligi uchun panic'ni ushlaymiz.
			defer func() {
				<-sem
				if r := recover(); r != nil {
					log.Printf("⚠️ panic recovered (update handler): %v\n%s", r, debug.Stack())
				}
			}()
			handler.Handle(tg, upd)
		}()
	}
}
