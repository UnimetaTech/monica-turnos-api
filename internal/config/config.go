package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	Port              string
	FrontendOrigin    string
	N8NSyncWebhookURL string
	N8NSyncSecret     string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		Port:              getenvDefault("PORT", "8080"),
		FrontendOrigin:    getenvDefault("FRONTEND_ORIGIN", "http://localhost:5173"),
		N8NSyncWebhookURL: os.Getenv("N8N_SYNC_WEBHOOK_URL"),
		N8NSyncSecret:     os.Getenv("N8N_SYNC_SECRET"),
	}
}

func getenvDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
