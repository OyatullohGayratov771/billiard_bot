package app

import (
	"billiard_bot/internal/auth"
	"billiard_bot/internal/player"
)

type App struct {
	PlayerService *player.Service
	AuthService   *auth.Service
}
