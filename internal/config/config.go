package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	TelegramToken         string
	WebhookURL            string
	LogLevel              string
	WordsFile             string
	GrammarQuestionsFile  string
	GrammarCategoriesFile string
	SchedulerTime         string // daily summary send time, format "HH:MM"
	Timezone              string // IANA timezone name
	MaxRoundsPerDay       int    // max daily-task rounds a user can run per day
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		TelegramToken:         os.Getenv("TELEGRAM_TOKEN"),
		WebhookURL:            os.Getenv("WEBHOOK_URL"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		WordsFile:             getEnv("WORDS_FILE", "data/words.json"),
		GrammarQuestionsFile:  getEnv("GRAMMAR_QUESTIONS_FILE", "data/grammar.json"),
		GrammarCategoriesFile: getEnv("GRAMMAR_CATEGORIES_FILE", "data/grammar_categories.json"),
		SchedulerTime:         getEnv("SCHEDULER_TIME", "21:00"),
		Timezone:              getEnv("TIMEZONE", "Europe/Moscow"),
		MaxRoundsPerDay:       getEnvInt("MAX_ROUNDS_PER_DAY", 5),
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

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}
