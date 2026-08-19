package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"live-service/internal/config"
	"live-service/internal/db"
	"live-service/internal/handler"
	"live-service/internal/repository"
	"live-service/internal/streamer"
)

func main() {
	config.LoadConfig()
	cfg := config.AppConfig

	database, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("❌ DB ulanmadi: %v", err)
	}
	defer database.Close()

	branchRepo := repository.NewBranchRepo(database)
	tableRepo := repository.NewTableRepo(database)
	mgr := streamer.New(cfg.HLSDir, cfg.RecordingsDir)

	h := handler.New(branchRepo, tableRepo, mgr, cfg.InternalToken, cfg.BaseURL)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("🛑 To'xtatilmoqda — barcha live oqimlar yopilmoqda...")
		mgr.StopAll()
		_ = srv.Close()
	}()

	log.Printf("🚀 live-service :%s portda ishga tushdi", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Server xatosi: %v", err)
	}
}
