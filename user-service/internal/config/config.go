package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseDSN string
	Superadmins []int64
	Port        string
}

var AppConfig Config

func LoadConfig() {
	_ = godotenv.Load()

	AppConfig = Config{
		DatabaseDSN: mustEnv("DATABASE_DSN"),
		Superadmins: parseSuperadmins(getEnv("SUPERADMIN_IDS", "")),
		Port:        getEnv("USER_SERVICE_PORT", "8081"),
	}
	log.Println("✅ user-service config yuklandi")
}

func parseSuperadmins(s string) []int64 {
	var ids []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
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
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
