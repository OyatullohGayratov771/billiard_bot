package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
}

var AppConfig Config

func Load() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Failed to load .env file")
	}
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, relying on environment variables")
	}

	AppConfig = Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        os.Getenv("PORT"),
	}
}
