package main

import (
	"log"
	"net/http"

	"user-service/internal/config"
	"user-service/internal/db"
	"user-service/internal/handler"
	"user-service/internal/repository"
	"user-service/internal/service"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	userRepo := repository.NewUserRepo(database)
	auditRepo := repository.NewAuditRepo(database)
	userSvc := service.NewUserService(userRepo, auditRepo, cfg.Superadmins)

	h := handler.New(userSvc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Printf("🚀 user-service %s portda ishga tushdi", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}
