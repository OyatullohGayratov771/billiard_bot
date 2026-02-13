package app

import (
	"billiard_bot/internal/auth"
	"billiard_bot/internal/player"

	"github.com/jmoiron/sqlx"
)

func New(db *sqlx.DB) *App {
	playerRepo := player.NewRepository(db)
	authRepo := auth.NewRepository(db)

	playerService := player.NewService(playerRepo)
	authService := auth.NewService(authRepo)

	return &App{
		PlayerService: playerService,
		AuthService:   authService,
	}
}
