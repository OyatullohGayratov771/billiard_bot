package main

import (
	"log"
	"net/http"

	"billiard_bot/config"
	"billiard_bot/internal/app"
	httpapi "billiard_bot/internal/http"
	"billiard_bot/pkg/db"
)

func main() {
	cfg := config.Load()

	database := db.Connect(cfg.DatabaseURL)
	appContainer := app.New(database)

	router := httpapi.NewRouter(appContainer)

	log.Println("Server running on :" + cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}
