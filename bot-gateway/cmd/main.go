package main

import (
	"log"

	"bot-gateway/internal/bot"
	"bot-gateway/internal/config"
	"bot-gateway/internal/db"
	"bot-gateway/internal/repository"
	"bot-gateway/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Config yuklash
	config.LoadConfig()
	cfg := config.AppConfig

	// PostgreSQL ulanish
	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	// Migration
	if err := db.Migrate(database); err != nil {
		log.Fatalf("❌ Migration xatosi: %v", err)
	}

	// Filiallar va stollarni seed qilish
	if err := db.SeedBranches(database); err != nil {
		log.Printf("⚠️  Seed xatosi: %v", err)
	}

	// Repository'lar
	userRepo := repository.NewUserRepo(database)
	branchRepo := repository.NewBranchRepo(database)
	tableRepo := repository.NewTableRepo(database)
	sessionRepo := repository.NewSessionRepo(database)
	clipRepo := repository.NewClipRepo(database)
	auditRepo := repository.NewAuditRepo(database)

	// Service'lar
	userSvc := service.NewUserService(userRepo, auditRepo, cfg.Superadmins)
	tableSvc := service.NewTableService(tableRepo, branchRepo, sessionRepo, auditRepo)
	clipSvc := service.NewClipService(clipRepo, branchRepo, tableRepo, auditRepo)

	// Handler
	handler := bot.NewHandler(
		userRepo, branchRepo, tableRepo, sessionRepo, clipRepo, auditRepo,
		userSvc, tableSvc, clipSvc,
	)

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
