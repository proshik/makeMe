package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	TelegramToken string
	WebhookURL    string
	LogLevel      string
	WordsFile     string
	SchedulerTime string // daily summary send time, format "HH:MM"
	Timezone      string // IANA timezone name
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		WebhookURL:    os.Getenv("WEBHOOK_URL"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		WordsFile:     getEnv("WORDS_FILE", "data/words.json"),
		SchedulerTime: getEnv("SCHEDULER_TIME", "21:00"),
		Timezone:      getEnv("TIMEZONE", "Europe/Moscow"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
