package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseDSN      string
	Port             string
	JWTSecret        string
	BaseURL          string
	BotToken         string
	TournamentSvcURL string
	ClipSvcURL       string
	InternalToken    string
}

var AppConfig Config

func LoadConfig() {
	_ = godotenv.Load()
	AppConfig = Config{
		DatabaseDSN:      mustEnv("DATABASE_DSN"),
		Port:             getEnv("WEB_SERVICE_PORT", "8085"),
		JWTSecret:        mustEnv("JWT_SECRET"),
		BaseURL:          getEnv("WEB_BASE_URL", "https://billiardking.uz"),
		BotToken:         mustEnv("TELEGRAM_TOKEN"),
		TournamentSvcURL: getEnv("TOURNAMENT_SVC_URL", "http://tournament-service:8084"),
		ClipSvcURL:       getEnv("CLIP_SVC_URL", "http://clip-service:8083"),
		InternalToken:    getEnv("INTERNAL_TOKEN", ""),
	}
	log.Println("✅ web-service config yuklandi")
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ Majburiy env topilmadi: %s", key)
	}
	return val
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
