package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseDSN string
	Port        string
}

var AppConfig Config

func LoadConfig() {
	_ = godotenv.Load()
	AppConfig = Config{
		DatabaseDSN: mustEnv("DATABASE_DSN"),
		Port:        getEnv("TABLE_SERVICE_PORT", "8082"),
	}
	log.Println("✅ table-service config yuklandi")
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
