package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseDSN   string
	Port          string
	InternalToken string
	UploadsDir    string
}

var AppConfig Config

func LoadConfig() {
	_ = godotenv.Load()
	AppConfig = Config{
		DatabaseDSN:   mustEnv("DATABASE_DSN"),
		Port:          getEnv("SHOP_SERVICE_PORT", "8086"),
		InternalToken: getEnv("INTERNAL_TOKEN", ""),
		UploadsDir:    getEnv("UPLOADS_DIR", "/app/uploads"),
	}
	log.Println("✅ shop-service config yuklandi")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("❌ Majburiy env topilmadi: %s", key)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
