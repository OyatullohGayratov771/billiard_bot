package httpapi

import (
	"net/http"

	"billiard_bot/internal/app"
)

func NewRouter(app *app.App) http.Handler {
	mux := http.NewServeMux()

	// public
	mux.HandleFunc("/health", HealthHandler)

	// player
	mux.HandleFunc("/players/get-or-create", PlayerGetOrCreate(app))

	return mux
}
