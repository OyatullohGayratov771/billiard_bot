package main

import (
	"log"
	"net/http"

	"web-service/internal/config"
	"web-service/internal/db"
	"web-service/internal/handler"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	h := handler.New(database, cfg.JWTSecret, cfg.BaseURL, cfg.BotToken)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Printf("🚀 web-service %s portda ishga tushdi", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}
