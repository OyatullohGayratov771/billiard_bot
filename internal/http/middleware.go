package httpapi

import (
	"net/http"

	"billiard_bot/internal/auth"
)

type Middleware struct {
	authService *auth.Service
}

func NewMiddleware(authService *auth.Service) *Middleware {
	return &Middleware{authService: authService}
}

func (m *Middleware) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tgID := r.Header.Get("X-Telegram-ID")
		if tgID == "" {
			http.Error(w, "missing telegram id", http.StatusUnauthorized)
			return
		}

		isAdmin, err := m.authService.IsAdmin(123456789) // Example Telegram ID
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}