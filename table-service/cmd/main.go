package main

import (
	"log"
	"net/http"

	"table-service/internal/config"
	"table-service/internal/db"
	"table-service/internal/handler"
	"table-service/internal/repository"
	"table-service/internal/service"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("❌ Migration xatosi: %v", err)
	}

	if err := db.SeedBranches(database); err != nil {
		log.Printf("⚠️  Seed xatosi: %v", err)
	}

	branchRepo := repository.NewBranchRepo(database)
	tableRepo := repository.NewTableRepo(database)
	sessionRepo := repository.NewSessionRepo(database)
	auditRepo := repository.NewAuditRepo(database)

	tableSvc := service.NewTableService(tableRepo, branchRepo, sessionRepo, auditRepo)

	h := handler.New(tableSvc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Printf("🚀 table-service %s portda ishga tushdi", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}
