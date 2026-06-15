package main

import (
	"log"
	"net/http"

	"shop-service/internal/config"
	"shop-service/internal/db"
	"shop-service/internal/handler"
	"shop-service/internal/repository"
	"shop-service/internal/service"
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
	svc := service.New(repo, cfg.UploadsDir)
	h := handler.New(svc, cfg.InternalToken)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Printf("🚀 shop-service %s portda ishga tushdi", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}
