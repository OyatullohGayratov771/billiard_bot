package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Http struct {
		Host string
		Port string
	}
	


	ApiKey    string
	JwtSecret string
}

var AppConfig Config

func LoadConfig() {
	// .env optional
	_ = godotenv.Load()

	AppConfig = Config{
		Http: struct {
			Host string
			Port string
		}{
			Host: getEnv("HTTP_HOST", "0.0.0.0"),
			Port: getEnv("HTTP_PORT", "8080"),
		},

		ApiKey: os.Getenv("API_KEY"),
		JwtSecret: mustEnv("JWT_KEY"),
	}

	log.Println("config loaded successfully")
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("missing required env: %s", key)
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
