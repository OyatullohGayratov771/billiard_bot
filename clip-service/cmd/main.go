package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"clip-service/internal/config"
	"clip-service/internal/db"
	"clip-service/internal/handler"
	"clip-service/internal/recorder"
	"clip-service/internal/repository"
	"clip-service/internal/service"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	// Klip fayllari uchun papka
	clipsDir := getEnv("CLIPS_DIR", "/app/clips")
	if err := os.MkdirAll(clipsDir, 0755); err != nil {
		log.Fatalf("❌ Clips papka yaratilmadi: %v", err)
	}
	log.Printf("📁 Kliplar saqlash joyi: %s", clipsDir)

	clipRepo := repository.NewClipRepo(database)
	branchRepo := repository.NewBranchRepo(database)
	tableRepo := repository.NewTableRepo(database)
	auditRepo := repository.NewAuditRepo(database)

	rec := recorder.New(clipsDir)
	clipSvc := service.NewClipService(clipRepo, branchRepo, tableRepo, auditRepo, rec)

	// Har kecha soat 03:00 da 24 soatdan eski done kliplarni o'chiradi
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.Local)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			n := clipSvc.CleanupOldFiles()
			log.Printf("🗑️  Eski kliplar tozalandi: %d ta fayl o'chirildi", n)
		}
	}()

	h := handler.New(clipSvc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Printf("🚀 clip-service %s portda ishga tushdi", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
