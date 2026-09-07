package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
	AdminToken            string // bearer token for /admin/* endpoints; empty disables them

	// Web UI. Empty TelegramClientID disables the web interface entirely: the bot
	// keeps working, /login just isn't mounted. That mirrors pochitushki, where
	// web login and the bot are independent of each other.
	AppBaseURL           string // public origin of the web UI, e.g. https://makeme.example.com
	TelegramClientID     string // OAuth client id from Telegram
	TelegramClientSecret string // OAuth client secret from Telegram
	TelegramOAuthBaseURL string // OIDC issuer base, overridable so tests can point at a stub
	// TelegramExpectedIssuer is the exact iss the id_token must carry. It
	// defaults to Telegram's own issuer and is set separately from the base URL
	// on purpose: pointing the flow at a local login broker must be a deliberate
	// change to both, not something one edit quietly enables.
	TelegramExpectedIssuer string
	JWTSecret              string // base64, must decode to >=32 bytes for HS256
	JWTTTLHours            int    // lifetime of the auth_token cookie and the JWT inside it
	CookieSecure           bool   // false only for local http development
}

// WebEnabled reports whether the web UI has everything it needs to be mounted.
func (c *Config) WebEnabled() bool {
	return c.TelegramClientID != "" && c.TelegramClientSecret != "" &&
		c.JWTSecret != "" && c.AppBaseURL != ""
}

// RedirectURI is the OAuth callback Telegram sends the user back to.
func (c *Config) RedirectURI() string {
	return strings.TrimRight(c.AppBaseURL, "/") + "/auth/telegram/callback"
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
		AdminToken:            os.Getenv("ADMIN_TOKEN"),

		AppBaseURL:             os.Getenv("APP_BASE_URL"),
		TelegramClientID:       os.Getenv("TELEGRAM_CLIENT_ID"),
		TelegramClientSecret:   os.Getenv("TELEGRAM_CLIENT_SECRET"),
		TelegramOAuthBaseURL:   getEnv("TELEGRAM_OAUTH_BASE_URL", "https://oauth.telegram.org"),
		TelegramExpectedIssuer: getEnv("TELEGRAM_EXPECTED_ISSUER", "https://oauth.telegram.org"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		JWTTTLHours:            getEnvInt("JWT_TTL_HOURS", 24),
		CookieSecure:           getEnvBool("COOKIE_SECURE", true),
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

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
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
