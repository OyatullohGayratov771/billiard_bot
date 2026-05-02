package main

import (
	"log"
	"net/http"

	"tournament-service/internal/config"
	"tournament-service/internal/db"
	"tournament-service/internal/handler"
	"tournament-service/internal/repository"
	"tournament-service/internal/service"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	repo := repository.New(database)
	svc := service.New(database, repo)
	h := handler.New(svc, cfg.InternalToken)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Printf("🚀 tournament-service %s portda ishga tushdi", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}
