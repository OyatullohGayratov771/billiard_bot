package httpapi

import (
	"encoding/json"
	"net/http"

	"billiard_bot/internal/app"
)

type playerRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Name       string `json:"name"`
}

func PlayerGetOrCreate(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req playerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		player, err := app.PlayerService.GetOrCreate(req.TelegramID, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(player)
	}
}
