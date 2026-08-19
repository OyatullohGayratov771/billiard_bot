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
	HLSDir        string
	RecordingsDir string
	BaseURL       string
}

var AppConfig Config

func LoadConfig() {
	_ = godotenv.Load()
	AppConfig = Config{
		DatabaseDSN:   mustEnv("DATABASE_DSN"),
		Port:          getEnv("LIVE_SERVICE_PORT", "8087"),
		InternalToken: getEnv("INTERNAL_TOKEN", ""),
		HLSDir:        getEnv("HLS_DIR", "/app/hls"),
		RecordingsDir: getEnv("RECORDINGS_DIR", "/app/recordings"),
		BaseURL:       getEnv("WEB_BASE_URL", "https://billiardking.uz"),
	}
	log.Println("✅ live-service config yuklandi")
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
