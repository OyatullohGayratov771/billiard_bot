package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
	DatabaseDSN   string
	APIKey        string
	JwtSecret     string
	// Telegram IDs of superadmins (comma-separated in env)
	Superadmins []int64
}

var AppConfig Config

func LoadConfig() {
	_ = godotenv.Load()

	AppConfig = Config{
		TelegramToken: mustEnv("TELEGRAM_TOKEN"),
		DatabaseDSN:   mustEnv("DATABASE_DSN"),
		APIKey:        getEnv("API_KEY", ""),
		JwtSecret:     mustEnv("JWT_SECRET"),
		Superadmins:   parseSuperadmins(getEnv("SUPERADMIN_IDS", "")),
	}

	log.Println("✅ Config yuklandi")
}

func parseSuperadmins(s string) []int64 {
	var ids []int64
	if s == "" {
		return ids
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			log.Printf("⚠️  Noto'g'ri superadmin ID: %s", part)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ Majburiy env topilmadi: %s", key)
	}
	return val
}

func getEnv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}
